package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// safeNamePattern allows only DNS-compatible names.
	safeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}[a-z0-9]$`)

	// pathTraversalPattern detects path traversal attempts.
	pathTraversalPattern = regexp.MustCompile(`\.\./|\.\.\\`)

	// shellInjectionChars that should never appear in model names or identifiers.
	shellInjectionChars = regexp.MustCompile("[;|&`$(){}\\[\\]<>!]")

	// reservedEnvVars that specs should not override.
	reservedEnvVars = map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "SHELL": true,
		"LD_PRELOAD": true, "LD_LIBRARY_PATH": true,
	}
)

// Sanitize performs deep validation and sanitization of a ModelSpec
// beyond basic field validation. Catches security issues.
func (s *ModelSpec) Sanitize() error {
	if err := validateName(s.Name, "name"); err != nil {
		return err
	}

	if err := validateModelRef(s.Model); err != nil {
		return err
	}

	if s.Quantization != "" {
		if err := validateQuantization(s.Quantization); err != nil {
			return err
		}
	}

	for i, tool := range s.Tools {
		if err := validateToolSpec(tool, i); err != nil {
			return err
		}
	}

	if s.PromptTemplate != "" {
		if err := validatePromptTemplate(s.PromptTemplate); err != nil {
			return err
		}
	}

	for _, origin := range s.Security.AllowedOrigins {
		if err := validateOrigin(origin); err != nil {
			return err
		}
	}

	return nil
}

func validateName(name, field string) error {
	if !safeNamePattern.MatchString(name) {
		return fmt.Errorf("%s %q is not a valid DNS-compatible name (lowercase alphanumeric and hyphens only)", field, name)
	}
	return nil
}

func validateModelRef(model string) error {
	if pathTraversalPattern.MatchString(model) {
		return fmt.Errorf("model reference %q contains path traversal", model)
	}
	if shellInjectionChars.MatchString(model) {
		return fmt.Errorf("model reference %q contains disallowed characters", model)
	}
	if len(model) > 256 {
		return fmt.Errorf("model reference exceeds 256 characters")
	}
	return nil
}

func validateQuantization(q string) error {
	allowed := map[string]bool{
		"awq": true, "gptq": true, "squeezellm": true, "fp8": true,
		"q4_0": true, "q4_1": true, "q4_k_m": true, "q4_k_s": true,
		"q5_0": true, "q5_1": true, "q5_k_m": true, "q5_k_s": true,
		"q8_0": true, "q6_k": true,
	}
	if !allowed[strings.ToLower(q)] {
		return fmt.Errorf("unsupported quantization %q", q)
	}
	return nil
}

func validateToolSpec(tool ToolSpec, index int) error {
	if tool.Name == "" {
		return fmt.Errorf("tool[%d] has empty name", index)
	}
	if shellInjectionChars.MatchString(tool.Name) {
		return fmt.Errorf("tool[%d] name %q contains disallowed characters", index, tool.Name)
	}
	if tool.Endpoint != "" {
		if !strings.HasPrefix(tool.Endpoint, "http://") && !strings.HasPrefix(tool.Endpoint, "https://") {
			return fmt.Errorf("tool[%d] endpoint must be http:// or https://", index)
		}
		if pathTraversalPattern.MatchString(tool.Endpoint) {
			return fmt.Errorf("tool[%d] endpoint contains path traversal", index)
		}
	}
	return nil
}

func validatePromptTemplate(tmpl string) error {
	if len(tmpl) > 10000 {
		return fmt.Errorf("prompt_template exceeds 10000 character limit")
	}
	if shellInjectionChars.MatchString(tmpl) {
		return fmt.Errorf("prompt_template contains disallowed shell characters")
	}
	return nil
}

func validateOrigin(origin string) error {
	if origin == "*" {
		return nil
	}
	if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
		return fmt.Errorf("allowed_origin %q must be http:// or https:// or *", origin)
	}
	return nil
}
