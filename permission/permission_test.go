package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecker_Check(t *testing.T) {
	tests := []struct {
		name    string
		rules   map[string]string
		command string
		wantErr bool
	}{
		{
			name: "deny all by default",
			rules: map[string]string{
				"*": "deny",
			},
			command: "rm -rf /",
			wantErr: true,
		},
		{
			name: "allow specific command",
			rules: map[string]string{
				"*":     "deny",
				"git *": "allow",
			},
			command: "git status",
			wantErr: false,
		},
		{
			name: "deny specific command",
			rules: map[string]string{
				"*":        "allow",
				"rm -rf *": "deny",
			},
			command: "rm -rf /",
			wantErr: true,
		},
		{
			name: "last match wins - allow",
			rules: map[string]string{
				"*":             "deny",
				"kubectl *":     "deny",
				"kubectl get *": "allow",
			},
			command: "kubectl get pods",
			wantErr: false,
		},
		{
			name: "last match wins - deny",
			rules: map[string]string{
				"*":                "allow",
				"kubectl *":        "allow",
				"kubectl delete *": "deny",
			},
			command: "kubectl delete pod nginx",
			wantErr: true,
		},
		{
			name: "empty command",
			rules: map[string]string{
				"*": "allow",
			},
			command: "",
			wantErr: true,
		},
		{
			name: "whitespace command",
			rules: map[string]string{
				"*": "allow",
			},
			command: "   ",
			wantErr: true,
		},
		{
			name: "exact match",
			rules: map[string]string{
				"*":   "deny",
				"pwd": "allow",
			},
			command: "pwd",
			wantErr: false,
		},
		{
			name: "case sensitive",
			rules: map[string]string{
				"*":     "deny",
				"Git *": "allow",
			},
			command: "git status",
			wantErr: true, // "git" != "Git"
		},
		{
			name: "prevent injection - semicolon",
			rules: map[string]string{
				"*":      "deny",
				"ls *":   "allow",
				"whoami": "deny",
			},
			command: "ls; whoami",
			wantErr: true,
		},
		{
			name: "prevent injection - and",
			rules: map[string]string{
				"*":      "deny",
				"ls *":   "allow",
				"whoami": "deny",
			},
			command: "ls && whoami",
			wantErr: true,
		},
		{
			name: "prevent injection - pipe",
			rules: map[string]string{
				"*":      "deny",
				"ls *":   "allow",
				"grep *": "deny",
			},
			command: "ls | grep foo",
			wantErr: true,
		},
		{
			name: "allow complex pipe if all allowed",
			rules: map[string]string{
				"*":      "deny",
				"ls *":   "allow",
				"grep *": "allow",
			},
			command: "ls -l | grep foo",
			wantErr: false,
		},
		{
			name: "normalized command matching",
			rules: map[string]string{
				"*":     "deny",
				"ls -l": "allow",
			},
			command: "ls    -l",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.rules)
			err := checker.Check(tt.command)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChecker_IsAllowed(t *testing.T) {
	checker := NewChecker(map[string]string{
		"*":    "deny",
		"ls *": "allow",
	})

	assert.True(t, checker.IsAllowed("ls -la"))
	assert.False(t, checker.IsAllowed("rm -rf /"))
}

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	checker := NewChecker(rules)

	// All commands should be denied by default
	assert.False(t, checker.IsAllowed("ls"))
	assert.False(t, checker.IsAllowed("cat /etc/passwd"))
	assert.False(t, checker.IsAllowed("rm -rf /"))
}

func TestSafeReadOnlyRules(t *testing.T) {
	rules := SafeReadOnlyRules()
	checker := NewChecker(rules)

	// Read-only commands should be allowed
	assert.True(t, checker.IsAllowed("cat /etc/hosts"))
	assert.True(t, checker.IsAllowed("ls -la"))
	assert.True(t, checker.IsAllowed("pwd"))
	assert.True(t, checker.IsAllowed("grep pattern file.txt"))

	// Dangerous commands should be denied
	assert.False(t, checker.IsAllowed("rm -rf /"))
	assert.False(t, checker.IsAllowed("mv /etc/passwd /tmp/"))
	assert.False(t, checker.IsAllowed("chmod 777 /"))
}

func TestKubernetesReadOnlyRules(t *testing.T) {
	rules := KubernetesReadOnlyRules()
	checker := NewChecker(rules)

	// Read-only kubectl commands should be allowed
	assert.True(t, checker.IsAllowed("kubectl get pods"))
	assert.True(t, checker.IsAllowed("kubectl describe pod nginx"))
	assert.True(t, checker.IsAllowed("kubectl logs nginx"))
	assert.True(t, checker.IsAllowed("kubectl version"))

	// Write kubectl commands should be denied
	assert.False(t, checker.IsAllowed("kubectl delete pod nginx"))
	assert.False(t, checker.IsAllowed("kubectl apply -f deployment.yaml"))
	assert.False(t, checker.IsAllowed("kubectl exec -it nginx -- bash"))
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		command string
		want    bool
	}{
		{"*", "anything", true},
		{"git *", "git status", true},
		{"git *", "git", false},
		{"kubectl get *", "kubectl get pods", true},
		{"kubectl get *", "kubectl describe pods", false},
		{"*.txt", "file.txt", true},
		{"*.txt", "file.log", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.command, func(t *testing.T) {
			got, err := matchPattern(tt.pattern, tt.command)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
