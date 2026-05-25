package template

import (
	"strings"
)

func ParseDockerfile(content string, builder *TemplateBase) error {
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Handle line continuation
		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, "\\") + strings.TrimSpace(lines[i])
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		instruction := strings.ToUpper(parts[0])
		args := strings.TrimSpace(parts[1])

		switch instruction {
		case "FROM":
			builder.FromImage(args, nil)
		case "RUN":
			builder.RunCmd(args, nil)
		case "COPY", "ADD":
			copyParts := strings.Fields(args)
			if len(copyParts) >= 2 {
				src := copyParts[len(copyParts)-2]
				dest := copyParts[len(copyParts)-1]
				builder.Copy(src, dest, nil)
			}
		case "WORKDIR":
			builder.SetWorkdir(args)
		case "USER":
			builder.SetUser(args)
		case "ENV":
			kv := strings.SplitN(args, "=", 2)
			if len(kv) == 2 {
				builder.SetEnvs(map[string]string{strings.TrimSpace(kv[0]): strings.TrimSpace(kv[1])})
			}
		case "CMD", "ENTRYPOINT":
			builder.SetStartCmd(args, nil)
		}
	}
	return nil
}
