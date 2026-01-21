// Package permission implements glob-based command permission checking.
package permission

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"mvdan.cc/sh/v3/syntax"
)

// Action represents the permission action.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

// Rule represents a single permission rule.
type Rule struct {
	Pattern string
	Action  Action
}

// Checker checks command permissions against configured rules.
type Checker struct {
	rules []Rule
}

// NewChecker creates a new permission checker from a map of patterns to actions.
// The "*" pattern is processed first as the default rule, followed by other patterns in sorted order.
func NewChecker(patterns map[string]string) *Checker {
	c := &Checker{
		rules: make([]Rule, 0, len(patterns)),
	}

	// Add default "*" rule first if present
	if action, ok := patterns["*"]; ok {
		c.rules = append(c.rules, Rule{
			Pattern: "*",
			Action:  Action(strings.ToLower(action)),
		})
	}

	// Add other rules in sorted order
	otherPatterns := getSortedPatterns(patterns)
	for _, pattern := range otherPatterns {
		c.rules = append(c.rules, Rule{
			Pattern: pattern,
			Action:  Action(strings.ToLower(patterns[pattern])),
		})
	}

	return c
}

// getSortedPatterns returns all patterns except "*" in sorted order.
func getSortedPatterns(patterns map[string]string) []string {
	var result []string
	for pattern := range patterns {
		if pattern != "*" {
			result = append(result, pattern)
		}
	}
	sort.Strings(result)
	return result
}

// Check checks if a command is allowed.
// Returns nil if allowed, error with reason if denied.
func (c *Checker) Check(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("empty command")
	}

	// Parse the command into a list of sub-commands to prevent injection (e.g., cmd1; cmd2)
	parser := syntax.NewParser()
	f, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	var checkErr error
	syntax.Walk(f, func(node syntax.Node) bool {
		if checkErr != nil {
			return false
		}

		switch x := node.(type) {
		case *syntax.Stmt:
			// Check redirects for potential path escape or unauthorized writes
			for _, redir := range x.Redirs {
				if redir.Word != nil {
					target := c.nodeToString(redir.Word)
					// We don't have rules for redirects yet, but we can detect them.
					_ = target
				}
			}

		case *syntax.CallExpr:
			// Check command and arguments
			cmdStr := c.nodeToString(x)
			if cmdStr != "" {
				finalAction, matchedPattern := c.evaluateRules(cmdStr)
				if finalAction == ActionDeny {
					checkErr = c.denyError(matchedPattern, cmdStr)
					return false
				}
			}
		}
		return true
	})

	return checkErr
}

// nodeToString converts a syntax node back to its string representation.
func (c *Checker) nodeToString(node syntax.Node) string {
	var sb strings.Builder
	printer := syntax.NewPrinter()
	_ = printer.Print(&sb, node)
	return sb.String()
}

// evaluateRules checks all rules and returns the final action and matched pattern.
func (c *Checker) evaluateRules(command string) (Action, string) {
	action, pattern := ActionDeny, ""
	for _, rule := range c.rules {
		if matched, _ := matchPattern(rule.Pattern, command); matched {
			action, pattern = rule.Action, rule.Pattern
		}
	}
	return action, pattern
}

// denyError creates an appropriate error message for denied commands.
func (c *Checker) denyError(matchedPattern, command string) error {
	if matchedPattern == "" {
		return fmt.Errorf("command denied (no matching allow rule): %s", command)
	}
	return fmt.Errorf("command denied by rule '%s': %s", matchedPattern, command)
}

// matchPattern checks if a command matches a glob pattern.
func matchPattern(pattern, command string) (bool, error) {
	switch {
	case pattern == "*":
		return true, nil
	case !strings.Contains(pattern, "*"):
		return pattern == command, nil
	case strings.HasSuffix(pattern, "*"):
		return strings.HasPrefix(command, strings.TrimSuffix(pattern, "*")), nil
	default:
		return doublestar.Match(pattern, command)
	}
}

// IsAllowed is a convenience method that returns true if command is allowed.
func (c *Checker) IsAllowed(command string) bool {
	return c.Check(command) == nil
}

// DefaultRules returns a set of safe default rules.
func DefaultRules() map[string]string {
	return map[string]string{
		"*": "deny", // Deny all by default
	}
}

// SafeReadOnlyRules returns rules that allow read-only operations.
func SafeReadOnlyRules() map[string]string {
	return map[string]string{
		"*":        "deny",
		"cat *":    "allow",
		"head *":   "allow",
		"tail *":   "allow",
		"ls *":     "allow",
		"ls":       "allow",
		"pwd":      "allow",
		"whoami":   "allow",
		"date":     "allow",
		"echo *":   "allow",
		"grep *":   "allow",
		"find *":   "allow",
		"which *":  "allow",
		"env":      "allow",
		"uname *":  "allow",
		"uname":    "allow",
		"df *":     "allow",
		"df":       "allow",
		"du *":     "allow",
		"free *":   "allow",
		"free":     "allow",
		"uptime":   "allow",
		"ps *":     "allow",
		"ps":       "allow",
		"top -b *": "allow",
	}
}

// KubernetesReadOnlyRules returns rules for read-only Kubernetes operations.
func KubernetesReadOnlyRules() map[string]string {
	return map[string]string{
		"*":                     "deny",
		"kubectl get *":         "allow",
		"kubectl describe *":    "allow",
		"kubectl logs *":        "allow",
		"kubectl top *":         "allow",
		"kubectl version *":     "allow",
		"kubectl version":       "allow",
		"kubectl cluster-info":  "allow",
		"kubectl api-resources": "allow",
	}
}
