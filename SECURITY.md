# Security Configuration

## Container Security

### Process Health Monitoring

The MCP Architecture Service includes comprehensive health monitoring:

- **Health Check Script**: `/app/healthcheck.sh` monitors the MCP server process
- **Docker Health Check**: Configured with 30s intervals, 10s timeout, 3 retries
- **Kubernetes Probes**: Liveness, readiness, and startup probes for robust monitoring
- **Process Monitoring**: Uses `pgrep` to verify the MCP server process is running

### Minimal Privileges

The container runs with minimal required privileges:

- **Non-root User**: Runs as user `mcpuser` (UID 1001, GID 1001)
- **No Privilege Escalation**: `allowPrivilegeEscalation: false`
- **Dropped Capabilities**: All Linux capabilities dropped (`drop: ALL`)
- **No New Privileges**: Security option `no-new-privileges:true`

### Read-only File System

The container uses a read-only root filesystem for security:

- **Read-only Root**: `readOnlyRootFilesystem: true`
- **Writable Volumes**: Only `/app/tmp` and `/app/logs` are writable
- **Temporary Storage**: Uses `tmpfs` mounts with size limits and security options
- **No Executable Temp**: Temporary directories mounted with `noexec` option

### Resource Limits

Resource constraints prevent resource exhaustion attacks:

- **Memory Limits**: 256Mi limit, 64Mi request
- **CPU Limits**: 200m (0.2 CPU cores) limit, 50m request
- **Temporary Storage**: 100Mi limit for each writable directory
- **Log Rotation**: 10MB max log size, 3 file rotation

### Security Profiles

Advanced security profiles are applied:

- **Seccomp Profile**: Uses `RuntimeDefault` seccomp profile
- **Security Context**: Pod and container security contexts configured
- **AppArmor**: Compatible with AppArmor profiles (when available)
- **SELinux**: Compatible with SELinux policies (when available)

## Kubernetes Security

### Pod Security Standards

The deployment follows Kubernetes Pod Security Standards:

- **Restricted Profile**: Meets the "restricted" pod security standard
- **Security Context**: Comprehensive security context configuration
- **Service Account**: Disabled automatic service account token mounting
- **Network Policy**: Restrictive network policies applied

### Network Security

Network access is minimized:

- **No Network Ports**: MCP server uses stdio communication only
- **Network Policy**: Ingress/egress rules limit network access
- **DNS Only**: Only DNS traffic (port 53) allowed for egress
- **No Service Account**: Service account token mounting disabled

### Monitoring and Compliance

Security monitoring capabilities:

- **Health Checks**: Multiple probe types for comprehensive monitoring
- **Resource Monitoring**: CPU and memory usage tracking
- **Security Events**: Container security events logged
- **Compliance**: Meets CIS Kubernetes Benchmark recommendations

## Deployment Security

### Container Image Security

- **Multi-stage Build**: Minimal runtime image without build tools
- **Alpine Base**: Minimal attack surface with Alpine Linux
- **No Package Manager**: Runtime image has no package manager
- **Static Binary**: Go binary compiled with static linking

### Runtime Security

- **Process Isolation**: Container runs single process (MCP server)
- **File System Protection**: Read-only root filesystem prevents tampering
- **Resource Isolation**: cgroups enforce resource limits
- **Namespace Isolation**: Process, network, and filesystem namespaces

## Security Best Practices

### Development

- Regular security updates for base images
- Dependency vulnerability scanning
- Static code analysis for security issues
- Container image scanning before deployment

### Operations

- Regular health check monitoring
- Resource usage monitoring and alerting
- Security event logging and analysis
- Incident response procedures for security events

### Compliance

- Follows NIST container security guidelines
- Implements CIS Docker and Kubernetes benchmarks
- Meets OWASP container security recommendations
- Compatible with enterprise security policies