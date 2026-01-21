# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

1. **Do NOT** create a public GitHub issue for security vulnerabilities
2. Send an email to **security@flashcat.cloud** with:
   - A description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact assessment
   - Any suggested fixes (optional)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Initial Assessment**: Within 7 days, we will provide an initial assessment
- **Resolution Timeline**: We aim to resolve critical issues within 30 days
- **Disclosure**: We will coordinate with you on public disclosure timing

### Scope

The following are in scope for security reports:

- Authentication/Authorization bypass
- Remote code execution
- Command injection vulnerabilities
- Privilege escalation
- Data exposure or leakage
- WebSocket security issues
- Configuration security issues

### Out of Scope

- Denial of Service (DoS) attacks
- Social engineering
- Physical attacks
- Issues in third-party dependencies (report to the upstream project)

## Security Best Practices

When deploying Flashduty Runner:

1. **API Key Protection**
   - Store API keys securely (environment variables or config files with restricted permissions)
   - Never commit API keys to version control
   - Rotate API keys periodically

2. **Network Security**
   - Run the runner in a network-segmented environment
   - Use firewall rules to restrict outbound connections
   - Consider using a proxy for outbound traffic

3. **Permission Control**
   - Configure bash command allowlist/denylist appropriately
   - Use the principle of least privilege
   - Restrict workspace directory access

4. **Monitoring**
   - Monitor runner logs for suspicious activity
   - Set up alerts for failed authentication attempts
   - Review executed commands periodically

## Security Features

Flashduty Runner includes several security features:

- **Command Permission Control**: Glob-pattern based allowlist/denylist for bash commands
- **Workspace Isolation**: Operations are restricted to the configured workspace root
- **TLS Encryption**: All WebSocket connections use TLS by default
- **API Key Authentication**: Secure authentication with the Flashduty platform
