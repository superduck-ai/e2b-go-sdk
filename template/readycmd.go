package template

import "fmt"

type ReadyCmd struct {
	Command string
}

func (r *ReadyCmd) GetCmd() string {
	if r == nil {
		return ""
	}
	return r.Command
}

func WaitForPort(port int) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("ss -tuln | grep :%d", port)}
}

func WaitForURL(url string, statusCode ...int) *ReadyCmd {
	code := 200
	if len(statusCode) > 0 {
		code = statusCode[0]
	}
	return &ReadyCmd{Command: fmt.Sprintf("curl -s -o /dev/null -w \"%%{http_code}\" %s | grep -q \"%d\"", url, code)}
}

func WaitForProcess(processName string) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("pgrep %s > /dev/null", processName)}
}

func WaitForFile(filename string) *ReadyCmd {
	return &ReadyCmd{Command: fmt.Sprintf("[ -f %s ]", filename)}
}

func WaitForTimeout(timeoutMs int) *ReadyCmd {
	seconds := timeoutMs / 1000
	if seconds < 1 {
		seconds = 1
	}
	return &ReadyCmd{Command: fmt.Sprintf("sleep %d", seconds)}
}
