package server

import (
	"os"
	"os/exec"
	"testing"
)

// 获得Git的版本号
func gitVersion() string {
	version := "v1.0.0"

	cmd := exec.Command("git", "describe", "--tags", "--always")
	out, err := cmd.Output()
	if err == nil {
		version = string(out)
	}
	for i := 0; i < len(version); i++ {
		if version[i] == '\n' {
			version = version[:i]
			break
		}
	}

	return version
}

// 编译程序
func build(t *testing.T, out string, in string, env ...string) error {
	version := gitVersion()

	cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+version, "-o", out, in)
	env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, env...)
	cmd.Dir = "./"
	t.Log("Executing command: " + cmd.String())

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// 复制文件
func copyFile(t *testing.T, src, dst string) {
	t.Log("Copying file from " + src + " to " + dst)
	srcFile, err := os.Open(src)
	if err != nil {
		t.Error(err)
		return
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		t.Error(err)
		return
	}
	defer dstFile.Close()
	_, err = dstFile.ReadFrom(srcFile)
	if err != nil {
		t.Error(err)
		return
	}
}

type BuildConfig struct {
	GOOS   string
	GOARCH string
	Ext    string
}

func TestBuildAll(t *testing.T) {

	list := []BuildConfig{
		{"linux", "amd64", ""},
		{"windows", "amd64", ".exe"},
		{"darwin", "amd64", ""},
		{"linux", "386", ""},
		{"windows", "386", ".exe"},
		{"linux", "arm64", ""},
		{"windows", "arm64", ".exe"},
		{"darwin", "arm64", ""},
		{"linux", "arm", ""},
		{"windows", "arm", ".exe"},
	}

	for _, config := range list {
		t.Log("Building server for " + config.GOOS + "/" + config.GOARCH)
		err := build(t, "./bin/proxy_"+config.GOOS+"_"+config.GOARCH+config.Ext, "./cmd", "GOOS="+config.GOOS, "GOARCH="+config.GOARCH, "CGO_ENABLED=0")
		if err != nil {
			t.Error(err)
		}
	}
}
