# Contributing

## Directory Structure

This project follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

### `/cmd`

Main applications for this project. The directory name for each application should match the name of the executable you want to have.

### `/internal`

Private application and library code. This is the code you don't want others importing in their applications or libraries. This layout pattern is enforced by the Go compiler itself.

- `/internal/provider` - Core SAML provider implementation including server logic, database operations, and configuration

### `/.local`

Local development artifacts that are generated and not committed to version control. This directory is gitignored.

- `/.local/certs` - Generated SSL/TLS certificates for local development

### `/configs`

Configuration file templates or default configs. Put your configuration files here that should be committed to the repository.

### `/deployments`

IaaS, PaaS, system and container orchestration deployment configurations and templates.

- `/deployments/docker` - Docker and docker-compose related configuration files

### `/test`

Additional external test apps and test data.

- `/test/saml-service` - A test SAML service provider used for integration testing and development

## Commits

When contributing code, please follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification for commit messages.
