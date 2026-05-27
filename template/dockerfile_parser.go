package template

import (
	"encoding/json"
	"strings"
)

func parseDockerfile(content string, builder *TemplateBase) error {
	lines := strings.Split(content, "\n")
	fromCount := 0
	userChanged := false
	workdirChanged := false

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
			fromCount++
			baseImage := strings.Fields(args)
			if len(baseImage) > 0 {
				builder.FromImage(baseImage[0], nil)
				builder.SetUser("root")
				builder.SetWorkdir("/")
			}
		case "RUN":
			builder.RunCmd(args, nil)
		case "COPY", "ADD":
			parseCopyInstruction(args, builder)
		case "WORKDIR":
			builder.SetWorkdir(args)
			workdirChanged = true
		case "USER":
			builder.SetUser(args)
			userChanged = true
		case "ENV":
			kv := strings.SplitN(args, "=", 2)
			if len(kv) == 2 {
				builder.SetEnvs(map[string]string{strings.TrimSpace(kv[0]): strings.TrimSpace(kv[1])})
			}
		case "CMD", "ENTRYPOINT":
			builder.SetStartCmd(parseCommandInstruction(args), WaitForTimeout(20_000))
		}
	}

	if fromCount > 0 {
		if !userChanged {
			builder.SetUser("user")
		}
		if !workdirChanged {
			builder.SetWorkdir("/home/user")
		}
	}
	return nil
}

func parseCopyInstruction(args string, builder *TemplateBase) {
	copyParts := splitDockerfileArgs(args)
	if len(copyParts) < 2 {
		return
	}

	user := ""
	nonFlagParts := make([]string, 0, len(copyParts))
	for _, part := range copyParts {
		if strings.HasPrefix(part, "--chown=") {
			user = strings.TrimPrefix(part, "--chown=")
			continue
		}
		if strings.HasPrefix(part, "--") {
			continue
		}
		nonFlagParts = append(nonFlagParts, part)
	}
	if len(nonFlagParts) < 2 {
		return
	}

	src := nonFlagParts[0]
	dest := nonFlagParts[len(nonFlagParts)-1]
	builder.Copy(src, dest, &struct {
		User string
	}{User: user})
}

func parseCommandInstruction(args string) string {
	command := strings.TrimSpace(args)
	var commandParts []string
	if err := json.Unmarshal([]byte(command), &commandParts); err == nil {
		quotedParts := make([]string, len(commandParts))
		for i, part := range commandParts {
			quotedParts[i] = shellQuoteCommandArg(part)
		}
		return strings.Join(quotedParts, " ")
	}
	return command
}

func shellQuoteCommandArg(arg string) string {
	if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			strings.ContainsRune("_@%+=:,./-", r))
	}) == -1 {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

func splitDockerfileArgs(args string) []string {
	parts := make([]string, 0)
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range args {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
