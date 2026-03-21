package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func reloadGhostty() error {
	return exec.Command("pkill", "-USR2", "ghostty").Run()
}

const (
	configPath    = "~/.config/ghostty-shader-manager/ghostty-shader-manager.config.yaml"
	shadersFile   = "~/.config/ghostty/ghostty-shaders"
	managedHeader = "# managed by ghostty-shader-manager — do not edit manually"
)

type Shader struct {
	Path             string `yaml:"path"`
	Name             string `yaml:"name"`
	EnabledByDefault bool   `yaml:"enabled-by-default"`
}

type Config struct {
	Shaders []Shader `yaml:"shaders"`
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(expandPath(configPath))
	if err != nil {
		return nil, fmt.Errorf("could not read config at %s: %w", configPath, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}
	return &cfg, nil
}

// readShadersFile parses the ghostty-shaders file and returns a map of expanded_path -> enabled.
// Returns nil map (not an error) if the file does not exist.
func readShadersFile() (map[string]bool, error) {
	path := expandPath(shadersFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, managedHeader) {
			continue
		}
		enabled := true
		content := line
		if strings.HasPrefix(line, "# ") {
			enabled = false
			content = line[2:]
		}
		after, ok := strings.CutPrefix(strings.TrimSpace(content), "custom-shader = ")
		if !ok {
			continue
		}
		state[expandPath(strings.TrimSpace(after))] = enabled
	}
	return state, nil
}

func writeShadersFile(shaders []Shader, state map[string]bool) error {
	var sb strings.Builder
	sb.WriteString(managedHeader)
	sb.WriteString("\n\n")
	for _, s := range shaders {
		p := expandPath(s.Path)
		if state[p] {
			fmt.Fprintf(&sb, "custom-shader = %s\n", s.Path)
		} else {
			fmt.Fprintf(&sb, "# custom-shader = %s\n", s.Path)
		}
	}
	dest := expandPath(shadersFile)
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func cmdSync() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	existing, err := readShadersFile()
	if err != nil {
		return err
	}
	if existing == nil {
		existing = make(map[string]bool)
	}

	newState := make(map[string]bool)
	configPaths := make(map[string]bool)
	added, removed, kept := 0, 0, 0

	for _, s := range cfg.Shaders {
		p := expandPath(s.Path)
		configPaths[p] = true
		if en, found := existing[p]; found {
			newState[p] = en
			kept++
		} else {
			newState[p] = s.EnabledByDefault
			added++
		}
	}
	for p := range existing {
		if !configPaths[p] {
			removed++
		}
	}

	if err := writeShadersFile(cfg.Shaders, newState); err != nil {
		return err
	}
	fmt.Printf("synced: %d added, %d removed, %d kept\n", added, removed, kept)
	return reloadGhostty()
}

func cmdList() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	maxLen := 0
	for _, s := range cfg.Shaders {
		if len(s.Name) > maxLen {
			maxLen = len(s.Name)
		}
	}
	for _, s := range cfg.Shaders {
		fmt.Printf("%-*s  %s\n", maxLen, s.Name, s.Path)
	}
	return nil
}

func cmdStatus() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	state, err := readShadersFile()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("ghostty-shaders not found — run `ghostty-shader-manager sync` first")
	}
	for _, s := range cfg.Shaders {
		p := expandPath(s.Path)
		en, found := state[p]
		switch {
		case !found:
			fmt.Printf("[UNSYNCED]  %s\n", s.Name)
		case en:
			fmt.Printf("[ON]        %s\n", s.Name)
		default:
			fmt.Printf("[OFF]       %s\n", s.Name)
		}
	}
	return nil
}

func findShader(cfg *Config, name string) *Shader {
	for i := range cfg.Shaders {
		if cfg.Shaders[i].Name == name {
			return &cfg.Shaders[i]
		}
	}
	return nil
}

func cmdToggle(name string, enable bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	shader := findShader(cfg, name)
	if shader == nil {
		return fmt.Errorf("unknown shader %q — run `ghostty-shader-manager list` to see registered shaders", name)
	}
	state, err := readShadersFile()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("ghostty-shaders not found — run `ghostty-shader-manager sync` first")
	}
	p := expandPath(shader.Path)
	if _, found := state[p]; !found {
		return fmt.Errorf("shader %q not in ghostty-shaders — run `ghostty-shader-manager sync` first", name)
	}
	state[p] = enable
	if err := writeShadersFile(cfg.Shaders, state); err != nil {
		return err
	}
	verb := "disabled"
	if enable {
		verb = "enabled"
	}
	fmt.Printf("%s %s\n", verb, name)
	return reloadGhostty()
}

func cmdInteractive() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	state, err := readShadersFile()
	if err != nil {
		return err
	}
	if state == nil {
		state = make(map[string]bool)
	}

	var items []string
	for _, s := range cfg.Shaders {
		prefix := "[off] "
		if state[expandPath(s.Path)] {
			prefix = "[on]  "
		}
		items = append(items, prefix+s.Name)
	}

	cmd := exec.Command("fzf", "--multi",
		"--prompt", "shaders> ",
		"--header", "tab: mark  enter: toggle marked",
	)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 130 {
			return nil // Ctrl-C, no-op
		}
		return err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" || len(line) < 6 {
			continue
		}
		name := strings.TrimSpace(line[6:])
		shader := findShader(cfg, name)
		if shader == nil {
			continue
		}
		p := expandPath(shader.Path)
		state[p] = !state[p]
	}

	if err := writeShadersFile(cfg.Shaders, state); err != nil {
		return err
	}
	return reloadGhostty()
}

func main() {
	args := os.Args[1:]

	var err error
	switch {
	case len(args) == 0:
		err = cmdInteractive()
	case args[0] == "sync":
		err = cmdSync()
	case args[0] == "list":
		err = cmdList()
	case args[0] == "status":
		err = cmdStatus()
	case args[0] == "on" && len(args) == 2:
		err = cmdToggle(args[1], true)
	case args[0] == "off" && len(args) == 2:
		err = cmdToggle(args[1], false)
	default:
		fmt.Fprintln(os.Stderr, "usage: ghostty-shader-manager [sync|list|status|on <shader>|off <shader>]")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
