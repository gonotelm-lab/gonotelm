package deptest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

func TestOpenSandbox(t *testing.T) {
	ctx := context.Background()

	config := opensandbox.ConnectionConfig{
		Domain:   "localhost:23080", // opensandbox-server 地址
		Protocol: "http",
		APIKey:   "123456", // 配置文件里 lifecycle 的 key
	}

	// 1) 创建沙箱（创建后会自动解析出 execd 端点并建好 ExecdClient）
	sbx, err := opensandbox.CreateSandbox(ctx, config,
		opensandbox.SandboxCreateOptions{
			Image: "alpine:3.23.5",
			ResourceLimits: opensandbox.ResourceLimits{
				"cpu":    "50m",
				"memory": "64Mi",
			},
		})
	if err != nil {
		log.Fatal(err)
	}
	defer sbx.Kill(ctx) // 关闭/删除沙箱

	// 2) 执行一个 bash 命令
	exec, err := sbx.RunCommand(ctx, "echo 'Hello from sandbox!' && ls -la / && pwd && cat /etc/os-release", nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("stdout:\n%s\n", exec.Text())
	if exec.ExitCode != nil {
		fmt.Printf("exit code: %d\n", *exec.ExitCode)
	}

	// 测试上传文件
	err = sbx.UploadFile(ctx, bytes.NewBuffer([]byte("hello world from you host")),
		opensandbox.UploadFileOptions{
			Metadata: opensandbox.FileMetadata{
				Path: "/tmp/youarehere",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	exec, err = sbx.RunCommand(ctx, "ls /tmp", nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("stdout:\n%s\n", exec.Text())
	if exec.ExitCode != nil {
		fmt.Printf("exit code: %d\n", *exec.ExitCode)
	}

	// 下载文件
	content, err := sbx.DownloadFile(ctx, "/tmp/youarehere", "")
	if err != nil {
		t.Fatal(err)
	}
	defer content.Close()
	data, err := io.ReadAll(content)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("file data = %s\n", data)

	
}
