package main

import (
	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/bramvdbogaerde/go-scp"
	"fmt"
	"os"
	"github.com/chenhg5/ecsDeploy/deploy"
	"github.com/chenhg5/go-utils/sms"
	"time"
	"github.com/chenhg5/go-utils/ini"
	"golang.org/x/crypto/ssh"
	"flag"
	"path"
)

var Sizes = make(map[string]int64, 0)

func main() {

	var configPath string
	flag.StringVar(&configPath, "config", "./config/config.ini", "config path")

	config, _ := ini.Get(configPath, "bugger")
	dayuCfg, _ := ini.Get(configPath, "dayu")
	ecsCfg, _ := ini.Get(configPath, "ecs")
	project := deploy.NewProject(ecsCfg)

	smser := sms.InitAlidayu(dayuCfg["key"], dayuCfg["secret"], dayuCfg["sign"], dayuCfg["code"])

	// 初始化Size
	ips := project.GetIps()
	for i := 0; i < len(ips); i++ {
		// 复制
		localFile := "/root/logcenter/" + ips[i] + "/" + config["local_file"]
		err := Copy(config["ssh_user"], config["ssh_key"], ips[i] + ":22", config["remote_file"], localFile, config["local_file_permission"])
		if err != nil {
			panic(err)
		}
		fileInfo, err := os.Stat(localFile)
		if err != nil {
			panic(err)
		}
		Sizes[ips[i]] = fileInfo.Size()
	}

	for true {
		// 每隔五分钟进行一次
		time.Sleep(time.Minute * 5)

		// 阿里云获取最新的host
		ips := project.GetIps()

		fmt.Println("ips", ips)

		// 从 host 拉取最新的error.log
		for i := 0; i < len(ips); i++ {
			go func(host string) {
				// 复制
				localFile := "/root/logcenter/" + host + "/" + config["local_file"]
				Copy("root", config["ssh_key"], host + ":22", config["remote_file"], localFile, config["local_file_permission"])
				// 检查大小
				if !CheckSize(localFile, host) {
					// 如果有增加就发通知
					Notify(smser, host, config["phone"])
				} else {
					fmt.Println("ok, no send")
				}
			}(ips[i])
		}
	}
}

func CheckSize(file string, host string) bool {
	fileInfo, _ := os.Stat(file)
	fmt.Println("checkSize", fileInfo.Size(), "host", host)
	if size, ok := Sizes[host]; ok {
		if fileInfo.Size() != size {
			Sizes[host] = fileInfo.Size()
			return false
		}
		return true
	} else {
		Sizes[host] = fileInfo.Size()
		return true
	}
}

func Notify(smser *sms.AlidayuSmSType, host string, phone string) {
	fmt.Println("send it", phone, "host", host)
	smser.SendAlidayuSMS(phone, host)
}

func Copy(user, key, host, remoteFile, localFile, permission string) error {
	clientConfig, _ := auth.PrivateKey(user, key, ssh.InsecureIgnoreHostKey())

	client := scp.NewClient(host, &clientConfig)

	// Connect to the remote server
	err := client.Connect()
	if err != nil {
		fmt.Println("Couldn't establisch a connection to the remote server ", err)
		return err
	}

	os.MkdirAll(path.Dir(localFile), 0666)
	f, _ := os.Create(localFile)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	defer f.Close()

	client.CopyFile(f, remoteFile, permission)

	return nil
}
