package template

import "fmt"

type ReadyCmd struct {
	Command string
}

func WaitForPort(port int) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("ss -tuln | grep :%d", port)}
}

func WaitForURL(url string, statusCode ...int) *ReadyCmd {
	code := 200
	if len(statusCode) > 0 {
		code = statusCode[0]
	}
	return &ReadyCmd{Command: fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %s | grep -q %d", url, code)}
}

func WaitForProcess(processName string) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("pgrep -f %s", processName)}
}

func WaitForFile(filename string) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("[ -f %s ]", filename)}
}

func WaitForTimeout(timeoutMs int) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("sleep %f", float64(timeoutMs)/1000.0)}
}
