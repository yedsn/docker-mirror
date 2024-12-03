package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config 结构体用于保存配置
type Config struct {
	Harbor struct {
		Domain   string `yaml:"domain"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Project  string `yaml:"project"`
	} `yaml:"harbor"`
	DockerRegistries []string `yaml:"dockerRegistries"`
}

// GetConfigPath 返回配置文件的路径
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("获取用户主目录出错: %v", err)
	}
	configDir := filepath.Join(homeDir, ".config", "docker-mirror")
	os.MkdirAll(configDir, 0755) // 创建配置目录
	return filepath.Join(configDir, "config.yaml")
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveConfig 将配置保存到 YAML 文件
func SaveConfig(configFile string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, data, 0644)
}

// Execute 执行一个 shell 命令并返回其输出
func Execute(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// 标记镜像
func TagImage(image string, targetImage string) error {
	_, err := Execute("docker", "tag", image, image)
	return err
}

// 清理镜像
func CleanImage(image string) error {
	_, err := Execute("docker", "rmi", image)
	return err
}

// Prompt 提示用户输入
func Prompt(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Configure 通过提示用户输入来配置工具
func Configure(configFile string) error {
	config := &Config{}

	config.Harbor.Domain = Prompt("请输入 Harbor 域名: ")
	config.Harbor.Username = Prompt("请输入 Harbor 用户名: ")
	config.Harbor.Password = Prompt("请输入 Harbor 密码: ")
	config.Harbor.Project = Prompt("请输入 Harbor 项目 (默认为 public): ")
	if config.Harbor.Project == "" {
		config.Harbor.Project = "public"
	}

	// 预设 DockerRegistries 的默认值
	config.DockerRegistries = []string{
		"docker.m.daocloud.io",
		"quay.m.daocloud.io",
		"k8s.m.daocloud.io",
	}

	return SaveConfig(configFile, config)
}

// PrintHelp 打印帮助信息
func PrintHelp() {
	fmt.Println("用法: docker-mirror <command> [image]")
	fmt.Println("eg: docker pull bitnami/postgresql:11.14.0-debian-10-r22")
	fmt.Println("")
	fmt.Println("command:")
	fmt.Println("")
	fmt.Println("  config       初始化配置")
	fmt.Println("")
	fmt.Println("  pull         拉取镜像到本地，并推送到 Harbor 仓库")
	fmt.Println("               注意: 请不要在镜像名称中添加域名")
	fmt.Println("")
	fmt.Println("  pull-local   仅拉取镜像到本地，不推送到 Harbor 仓库")
	fmt.Println("               注意: 请不要在镜像名称中添加域名")
	fmt.Println("")
	fmt.Println("  push         推送本地镜像到 Harbor 仓库")
	fmt.Println("               注意: 请不要在镜像名称中添加域名")
	fmt.Println("")
	fmt.Println("  help         显示帮助信息")
	fmt.Println("")
	fmt.Println("注意: 如果需要使用自签名证书的私有仓库，请在 /etc/docker/daemon.json 中配置：")
	fmt.Println(`{
		"insecure-registries": ["your-registry-domain:port"]
	}`)
}

func main() {
	if len(os.Args) < 2 {
		PrintHelp()
		return
	}

	command := os.Args[1]
	configPath := GetConfigPath()

	switch command {
	case "config":
		if err := Configure(configPath); err != nil {
			log.Fatalf("配置出错: %v", err)
		}
		fmt.Println("配置保存成功。")
	case "pull":
		if len(os.Args) != 3 {
			fmt.Println("用法: docker-mirror pull <镜像>")
			return
		}

		image := os.Args[2]
		sourceImage := image
		part := strings.Split(image, "/")

		// 加载配置
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置出错: %v", err)
		}

		// 如果镜像名称中没有斜杠，则默认视为 library/镜像名称
		if len(part) == 1 {
			part = append([]string{"library"}, part[0])
		}

		targetImage := fmt.Sprintf("%s/%s/%s", config.Harbor.Domain, config.Harbor.Project, part[len(part)-1])
		harborImage := targetImage

		var pullErr error
		var sourceRegistry string

		// 首先尝试从Harbor拉取
		fmt.Printf("正在从Harbor拉取镜像 %s\n", harborImage)
		if output, err := Execute("docker", "pull", harborImage); err == nil {

			// 标记为镜像名
			fmt.Printf("正在将镜像 %s 标记为 %s\n", harborImage, image)
			if output, err := Execute("docker", "tag", harborImage, image); err != nil {
				log.Fatalf("标记镜像出错: %v\n%s", err, output)
			}

			// 清理本地镜像
			fmt.Printf("正在清理本地镜像 %s\n", harborImage)
			if err := CleanImage(harborImage); err != nil {
				log.Fatalf("清理本地镜像出错: %v", err)
			}
			pullErr = nil
			sourceRegistry = config.Harbor.Domain
			sourceImage = harborImage
		} else {
			fmt.Printf("从Harbor拉取失败: %v\n%s", err, output)
			// 如果从Harbor拉取失败，尝试其他镜像源
			if len(config.DockerRegistries) == 0 {
				// 如果 DockerRegistries 为空，则直接拉取不带域名的镜像
				fmt.Printf("正在从DockerHub拉取镜像 %s\n", sourceImage)
				if output, err := Execute("docker", "pull", sourceImage); err != nil {
					fmt.Printf("拉取镜像出错: %v\n%s", err, output)
					pullErr = err
				} else {
					pullErr = nil
					sourceRegistry = ""
				}
			} else {
				for _, registry := range config.DockerRegistries {
					// 从配置的 Docker 镜像仓库地址拉取镜像
					fmt.Printf("正在从 %s 拉取镜像 %s\n", registry, sourceImage)
					registryImage := fmt.Sprintf("%s/%s", registry, sourceImage)
					if output, err := Execute("docker", "pull", registryImage); err != nil {
						fmt.Printf("拉取镜像出错: %v\n%s", err, output)
						pullErr = err
					} else {
						// 标记为镜像名
						fmt.Printf("正在将镜像 %s 标记为 %s\n", registryImage, image)
						if output, err := Execute("docker", "tag", registryImage, image); err != nil {
							log.Fatalf("标记镜像出错: %v\n%s", err, output)
						}

						// 清理本地镜像
						fmt.Printf("正在清理本地镜像 %s\n", registryImage)
						if err := CleanImage(registryImage); err != nil {
							log.Fatalf("清理本地镜像出错: %v", err)
						}

						pullErr = nil
						sourceRegistry = registry
						sourceImage = registryImage
						break
					}
				}
			}
		}

		if pullErr != nil {
			log.Fatalf("从所有配置的镜像源拉取镜像均失败")
		}

		// 如果不是从Harbor拉取的，需要标记并推送到Harbor
		if sourceRegistry != config.Harbor.Domain {
			// 将镜像标记为目标域名
			fmt.Printf("正在将镜像 %s 标记为 %s\n", image, targetImage)
			if output, err := Execute("docker", "tag", image, targetImage); err != nil {
				log.Fatalf("标记镜像出错: %v\n%s", err, output)
			}

			// 登录到 Harbor 仓库
			fmt.Printf("正在登录到 Harbor 仓库 %s\n", config.Harbor.Domain)
			if output, err := Execute("docker", "login", config.Harbor.Domain, "-u", config.Harbor.Username, "-p", config.Harbor.Password); err != nil {
				log.Fatalf("登录 Harbor 出错: %v\n%s", err, output)
			}

			// 推送镜像到 Harbor 仓库
			fmt.Printf("正在推送镜像 %s\n", targetImage)
			if output, err := Execute("docker", "push", targetImage); err != nil {
				log.Fatalf("推送镜像出错: %v\n%s", err, output)
			}

			// 清理本地镜像
			fmt.Printf("正在清理本地镜像 %s\n", targetImage)
			if err := CleanImage(targetImage); err != nil {
				log.Fatalf("清理本地镜像出错: %v", err)
			}
		}

		fmt.Println("镜像成功同步！")

	case "pull-local":
		if len(os.Args) != 3 {
			fmt.Println("用法: docker-mirror pull-local <镜像>")
			return
		}

		image := os.Args[2]
		sourceImage := image

		// 加载配置
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置出错: %v", err)
		}

		var pullErr error
		if len(config.DockerRegistries) == 0 {
			// 如果 DockerRegistries 为空，则直接拉取不带域名的镜像
			fmt.Printf("正在从DockerHub拉取镜像 %s\n", sourceImage)
			if output, err := Execute("docker", "pull", sourceImage); err != nil {
				fmt.Printf("拉取镜像出错: %v\n%s", err, output)
				pullErr = err
			} else {
				// 标记为镜像名
				fmt.Printf("正在将镜像 %s 标记为 %s\n", sourceImage, image)
				if output, err := Execute("docker", "tag", sourceImage, image); err != nil {
					log.Fatalf("标记镜像出错: %v\n%s", err, output)
				}

				// 清理本地镜像
				fmt.Printf("正在清理本地镜像 %s\n", sourceImage)
				if err := CleanImage(sourceImage); err != nil {
					log.Fatalf("清理本地镜像出错: %v", err)
				}
				pullErr = nil
			}
		} else {
			for _, registry := range config.DockerRegistries {
				// 从配置的 Docker 镜像仓库地址拉取镜像
				fmt.Printf("正在从 %s 拉取镜像 %s\n", registry, sourceImage)
				if output, err := Execute("docker", "pull", fmt.Sprintf("%s/%s", registry, sourceImage)); err != nil {
					fmt.Printf("拉取镜像出错: %v\n%s", err, output)

					// 标记为镜像名
					fmt.Printf("正在将镜像 %s 标记为 %s\n", fmt.Sprintf("%s/%s", registry, sourceImage), image)
					if output, err := Execute("docker", "tag", fmt.Sprintf("%s/%s", registry, sourceImage), image); err != nil {
						log.Fatalf("标记镜像出错: %v\n%s", err, output)
					}

					// 清理本地镜像
					fmt.Printf("正在清理本地镜像 %s\n", fmt.Sprintf("%s/%s", registry, sourceImage))
					if err := CleanImage(fmt.Sprintf("%s/%s", registry, sourceImage)); err != nil {
						log.Fatalf("清理本地镜像出错: %v", err)
					}
					pullErr = err
				} else {
					pullErr = nil
					break
				}
			}
		}

		if pullErr != nil {
			log.Fatalf("从所有配置的 DockerRegistry 拉取镜像均失败")
		}

		fmt.Println("您的镜像已成功拉取到本地！")
	case "push":
		if len(os.Args) != 3 {
			fmt.Println("用法: docker-mirror push <镜像>")
			return
		}

		image := os.Args[2]

		// 加载配置
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置出错: %v", err)
		}

		// 解析镜像名称
		part := strings.Split(image, "/")
		if len(part) == 1 {
			part = append([]string{"library"}, part[0])
		}

		// 构建目标镜像名称
		targetImage := fmt.Sprintf("%s/%s/%s", config.Harbor.Domain, config.Harbor.Project, part[len(part)-1])

		// 将镜像标记为目标域名
		fmt.Printf("正在将镜像 %s 标记为 %s\n", image, targetImage)
		if output, err := Execute("docker", "tag", image, targetImage); err != nil {
			log.Fatalf("标记镜像出错: %v\n%s", err, output)
		}

		// 登录到 Harbor 仓库
		fmt.Printf("正在登录到 Harbor 仓库 %s\n", config.Harbor.Domain)
		if output, err := Execute("docker", "login", config.Harbor.Domain, "-u", config.Harbor.Username, "-p", config.Harbor.Password); err != nil {
			log.Fatalf("登录 Harbor 出错: %v\n%s", err, output)
		}

		// 推送镜像到 Harbor 仓库
		fmt.Printf("正在推送镜像 %s\n", targetImage)
		if output, err := Execute("docker", "push", targetImage); err != nil {
			log.Fatalf("推送镜像出错: %v\n%s", err, output)
		}

		// 清理本地标记的镜像
		fmt.Printf("正在清理本地镜像 %s\n", targetImage)
		if err := CleanImage(targetImage); err != nil {
			log.Fatalf("清理本地镜像出错: %v", err)
		}

		fmt.Println("镜像成功推送到Harbor！")
	case "help":
		PrintHelp()
	default:
		fmt.Println("unknown:", command)
		PrintHelp()
	}
}
