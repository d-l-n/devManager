package testutil

import (
	"os/exec"
	"runtime"
)

// PingCmd returns a long-running ping command suitable for tests.
func PingCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("ping", "-n", "30", "127.0.0.1")
	}
	return exec.Command("ping", "-c", "30", "127.0.0.1")
}

// ExitCmd returns a command that exits with the given code.
func ExitCmd(code int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "exit", itoa(code))
	}
	return exec.Command("sh", "-c", "exit "+itoa(code))
}

// EchoCmd returns a command that echoes a string.
func EchoCmd(msg string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "echo", msg)
	}
	return exec.Command("sh", "-c", "echo "+msg)
}

// EchoEnvCmd returns a command that echoes an environment variable.
func EchoEnvCmd(varName string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "echo", varName+"=%"+varName+"%")
	}
	return exec.Command("sh", "-c", "echo "+varName+"=$"+varName)
}

// ExitCmdStr returns the shell command string that exits with the given code.
func ExitCmdStr(code int) string {
	if runtime.GOOS == "windows" {
		return "cmd /c exit " + itoa(code)
	}
	return "exit " + itoa(code)
}

// EchoCmdStr returns the shell command string that echoes a message.
func EchoCmdStr(msg string) string {
	if runtime.GOOS == "windows" {
		return "cmd /c echo " + msg
	}
	return "echo " + msg
}

// PingCmdStr returns the shell command string for a long-running ping.
func PingCmdStr() string {
	if runtime.GOOS == "windows" {
		return "ping -n 30 127.0.0.1"
	}
	return "ping -c 30 127.0.0.1"
}

// SlowEchoCmdStr returns a shell command that waits roughly delaySec seconds
// then echoes msg. Linux: sleep N && echo M (ping -c is 1/sec — too slow).
// Windows: ping -n N+1 127.0.0.1 > nul, ~1s for ping, then echo.
func SlowEchoCmdStr(delaySec int, msg string) string {
	if runtime.GOOS == "windows" {
		return "ping -n " + itoa(delaySec+1) + " 127.0.0.1 > nul && echo " + msg
	}
	return "sleep " + itoa(delaySec) + " && echo " + msg
}

// EchoEnvCmdStr returns a command string that echoes an env var named varName.
func EchoEnvCmdStr(varName string) string {
	if runtime.GOOS == "windows" {
		return "cmd /c echo " + varName + "=%" + varName + "%"
	}
	return "echo " + varName + "=$" + varName
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
