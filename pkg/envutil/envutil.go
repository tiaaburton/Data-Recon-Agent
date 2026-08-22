package envutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnvFile reads a key=value env file and injects unset variables into os.Environ.
func LoadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

// PromptString asks the user for a text input with an optional default.
func PromptString(reader *bufio.Reader, promptText, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", promptText, defaultValue)
	} else {
		fmt.Printf("%s: ", promptText)
	}
	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// PromptChoice displays numbered choices and returns the selected index (1-based).
func PromptChoice(reader *bufio.Reader, header string, choices []string, defaultChoice int) int {
	fmt.Printf("\n%s\n", header)
	for i, c := range choices {
		marker := " "
		if i+1 == defaultChoice {
			marker = "*"
		}
		fmt.Printf("  [%d]%s %s\n", i+1, marker, c)
	}
	fmt.Printf("Select choice [1-%d] (default %d): ", len(choices), defaultChoice)
	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultChoice
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultChoice
	}
	var selected int
	if _, err := fmt.Sscanf(input, "%d", &selected); err == nil && selected >= 1 && selected <= len(choices) {
		return selected
	}
	return defaultChoice
}

// UpdateEnvLocal updates or appends key-value pairs into .env.local.
func UpdateEnvLocal(filepath string, updates map[string]string) error {
	var lines []string
	existingKeys := make(map[string]bool)

	file, err := os.Open(filepath)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					if newVal, exists := updates[key]; exists {
						line = fmt.Sprintf("%s=\"%s\"", key, newVal)
						existingKeys[key] = true
					}
				}
			}
			lines = append(lines, line)
		}
		file.Close()
	}

	// Append any new keys not found in the original file
	for k, v := range updates {
		if !existingKeys[k] {
			lines = append(lines, fmt.Sprintf("%s=\"%s\"", k, v))
		}
	}

	return os.WriteFile(filepath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
