package app

import (
	"fmt"
	"os/exec"
	"strings"
)

type PackageManagerInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Available   bool   `json:"available"`
}

type PackageGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Packages    []string `json:"packages"`
}

func detectPackageManagers() []PackageManagerInfo {
	items := []PackageManagerInfo{
		{Name: "apt", DisplayName: "APT (Ubuntu/Debian)"},
		{Name: "dnf", DisplayName: "DNF (Fedora/RHEL)"},
		{Name: "yum", DisplayName: "YUM (CentOS/RHEL)"},
	}
	for i := range items {
		_, err := exec.LookPath(items[i].Name)
		items[i].Available = err == nil
	}
	return items
}

func activePackageManager() string {
	for _, name := range []string{"apt", "dnf", "yum"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func packageGroups() []PackageGroup {
	return []PackageGroup{
		{ID: "steam-runtime", Name: "Steam 运行库", Description: "常见 SteamCMD 服务端所需 32 位和基础运行库", Packages: []string{"lib32gcc-s1", "lib32stdc++6", "libc6-i386", "libcurl4-gnutls-dev", "libssl-dev", "libstdc++6"}},
		{ID: "unity-runtime", Name: "Unity 游戏运行库", Description: "Rust、7 Days to Die、Satisfactory 等 Unity/图形依赖", Packages: []string{"libsdl2-2.0-0", "libpulse0", "libfontconfig1", "libudev1", "libvulkan1", "libx11-6", "libxrandr2", "libxi6", "libgtk-3-0"}},
		{ID: "ark-runtime", Name: "ARK/大型游戏运行库", Description: "ARK、UE 服务端常见依赖和字体工具", Packages: []string{"libelf1", "libatomic1", "xz-utils", "zlib1g", "fonts-wqy-zenhei", "fonts-wqy-microhei"}},
		{ID: "build-tools", Name: "构建工具链", Description: "Forge/Spigot/源码构建和压缩解压常用工具", Packages: []string{"build-essential", "git", "curl", "wget", "tar", "gzip", "unzip", "xz-utils"}},
	}
}

func installCommand(pkg string) string {
	if pkg == "steamcmd" {
		return "mkdir -p /usr/local/steamcmd && cd /usr/local/steamcmd && curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz -o steamcmd.tar.gz && tar -xzf steamcmd.tar.gz && rm -f steamcmd.tar.gz && ln -sf /usr/local/steamcmd/steamcmd.sh /usr/local/bin/steamcmd"
	}
	pm := activePackageManager()
	if pm == "" {
		return ""
	}
	if pkg == "java" {
		pkg = "java17"
	}
	if strings.HasPrefix(pkg, "java") {
		version := strings.TrimPrefix(pkg, "java")
		return packageInstallCommand(pm, javaPackageName(pm, version))
	}
	if pkg == "tools" {
		return packageInstallCommand(pm, []string{"curl", "wget", "tar", "gzip", "unzip"})
	}
	for _, group := range packageGroups() {
		if group.ID == pkg {
			return packageInstallCommand(pm, packagesForManager(pm, group.Packages))
		}
	}
	return ""
}

func javaPackageName(pm, version string) []string {
	switch pm {
	case "apt":
		return []string{fmt.Sprintf("openjdk-%s-jre-headless", version)}
	case "dnf", "yum":
		return []string{fmt.Sprintf("java-%s-openjdk-headless", version)}
	default:
		return nil
	}
}

func packageInstallCommand(pm string, packages []string) string {
	packages = cleanPackages(packages)
	if len(packages) == 0 {
		return ""
	}
	joined := strings.Join(packages, " ")
	switch pm {
	case "apt":
		return "apt-get update && apt-get install -y " + joined
	case "dnf":
		return "dnf install -y " + joined
	case "yum":
		return "yum install -y " + joined
	default:
		return ""
	}
}

func packagesForManager(pm string, packages []string) []string {
	if pm == "apt" {
		return packages
	}
	out := []string{}
	for _, pkg := range packages {
		if strings.Contains(pkg, ":i386") || strings.HasPrefix(pkg, "lib32") || pkg == "libc6-i386" || strings.HasPrefix(pkg, "fonts-wqy") || pkg == "build-essential" {
			continue
		}
		out = append(out, pkg)
	}
	if len(out) == 0 {
		out = []string{"curl", "wget", "tar", "gzip", "unzip"}
	}
	return out
}

func cleanPackages(packages []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	return out
}
