package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DetectResult struct {
	DockerfilePath string
	TechStack      string
	Generated      bool
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func hasExtension(dir string, ext string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ext {
			return true
		}
	}
	return false
}

func FindOrGenerateDockerfile(repoDir string) (*DetectResult, error) {
	customDockerfiles := []string{
		"Dockerfile",
		"dockerfile",
		"Dockerfile.prod",
		".docker/Dockerfile",
		"build/Dockerfile",
		"deploy/Dockerfile",
	}

	for _, rel := range customDockerfiles {
		fullPath := filepath.Join(repoDir, rel)
		if fileExists(fullPath) {
			return &DetectResult{
				DockerfilePath: fullPath,
				TechStack:      "custom-dockerfile",
				Generated:      false,
			}, nil
		}
	}

	targetDockerfile := filepath.Join(repoDir, "Dockerfile.deploydock")

	if fileExists(filepath.Join(repoDir, "package.json")) {
		content := generateNodeDockerfile(repoDir)
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing node dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "node",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "requirements.txt")) ||
		fileExists(filepath.Join(repoDir, "Pipfile")) ||
		fileExists(filepath.Join(repoDir, "pyproject.toml")) ||
		fileExists(filepath.Join(repoDir, "setup.py")) {
		content := generatePythonDockerfile(repoDir)
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing python dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "python",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "go.mod")) || hasExtension(repoDir, ".go") {
		content := generateGoDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing go dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "go",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "Cargo.toml")) {
		content := generateRustDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing rust dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "rust",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "pom.xml")) {
		content := generateMavenDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing maven dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "java-maven",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "build.gradle")) || fileExists(filepath.Join(repoDir, "build.gradle.kts")) {
		content := generateGradleDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing gradle dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "java-gradle",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "composer.json")) || fileExists(filepath.Join(repoDir, "index.php")) {
		content := generatePHPDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing php dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "php",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "Gemfile")) {
		content := generateRubyDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing ruby dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "ruby",
			Generated:      true,
		}, nil
	}

	if fileExists(filepath.Join(repoDir, "index.html")) || hasExtension(repoDir, ".html") {
		content := generateStaticDockerfile()
		if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed writing static dockerfile: %w", err)
		}
		return &DetectResult{
			DockerfilePath: targetDockerfile,
			TechStack:      "static-web",
			Generated:      true,
		}, nil
	}

	content := generateFallbackDockerfile()
	if err := os.WriteFile(targetDockerfile, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed writing fallback dockerfile: %w", err)
	}
	return &DetectResult{
		DockerfilePath: targetDockerfile,
		TechStack:      "universal-static",
		Generated:      true,
	}, nil
}

func generateNodeDockerfile(dir string) string {
	hasBuild := false
	hasStart := false
	mainFile := ""
	pkgData, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err == nil {
		var pkg struct {
			Main    string            `json:"main"`
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(pkgData, &pkg) == nil {
			if _, ok := pkg.Scripts["build"]; ok {
				hasBuild = true
			}
			if _, ok := pkg.Scripts["start"]; ok {
				hasStart = true
			}
			mainFile = pkg.Main
		}
	}

	buildStep := ""
	if hasBuild {
		buildStep = "RUN npm run build\n"
	}

	var startCmd string
	if hasStart {
		startCmd = `CMD ["npm", "start"]`
	} else if mainFile != "" && fileExists(filepath.Join(dir, mainFile)) {
		startCmd = fmt.Sprintf(`CMD ["node", "%s"]`, mainFile)
	} else {
		startCmd = `CMD ["sh", "-c", "if [ -f server.js ]; then node server.js; elif [ -f index.js ]; then node index.js; elif [ -f app.js ]; then node app.js; elif [ -f src/index.js ]; then node src/index.js; elif [ -f src/server.js ]; then node src/server.js; elif [ -f src/app.js ]; then node src/app.js; else npx --yes serve -s . -l 8080; fi"]`
	}

	return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
ENV PORT=8080
ENV NODE_ENV=production
COPY package*.json ./
RUN npm install
COPY . .
%sEXPOSE 8080
%s
`, buildStep, startCmd)
}

func generatePythonDockerfile(dir string) string {
	return `FROM python:3.11-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
ENV PORT=8080
COPY requirements*.txt Pipfile* pyproject.toml* ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
COPY . .
EXPOSE 8080
CMD ["sh", "-c", "if [ -f app.py ]; then python app.py; elif [ -f main.py ]; then python main.py; elif [ -f server.py ]; then python server.py; elif [ -f wsgi.py ]; then python wsgi.py; else python -m http.server 8080; fi"]
`
}

func generateGoDockerfile() string {
	return `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN if [ -f go.mod ]; then go mod download; fi
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server .
ENV PORT=8080
EXPOSE 8080
CMD ["./server"]
`
}

func generateRustDockerfile() string {
	return `FROM rust:1.77-alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY Cargo.toml Cargo.lock* ./
COPY src ./src
RUN cargo build --release
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/target/release/* /app/bin/
ENV PORT=8080
EXPOSE 8080
CMD ["sh", "-c", "exec $(find /app/bin -type f -executable | head -n 1)"]
`
}

func generateMavenDockerfile() string {
	return `FROM maven:3.9-eclipse-temurin-17-alpine AS builder
WORKDIR /app
COPY pom.xml .
COPY src ./src
RUN mvn clean package -DskipTests
FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=builder /app/target/*.jar app.jar
ENV PORT=8080
EXPOSE 8080
CMD ["java", "-jar", "app.jar"]
`
}

func generateGradleDockerfile() string {
	return `FROM gradle:8.7-jdk17-alpine AS builder
WORKDIR /app
COPY build.gradle* settings.gradle* ./
COPY src ./src
RUN gradle build -x test --no-daemon
FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=builder /app/build/libs/*.jar app.jar
ENV PORT=8080
EXPOSE 8080
CMD ["java", "-jar", "app.jar"]
`
}

func generatePHPDockerfile() string {
	return `FROM php:8.3-apache
RUN sed -i 's/80/8080/g' /etc/apache2/ports.conf /etc/apache2/sites-available/*.conf
COPY . /var/www/html/
EXPOSE 8080
CMD ["apache2-foreground"]
`
}

func generateRubyDockerfile() string {
	return `FROM ruby:3.3-alpine
WORKDIR /app
RUN apk add --no-cache build-base
COPY Gemfile Gemfile.lock* ./
RUN bundle install
COPY . .
ENV PORT=8080
EXPOSE 8080
CMD ["sh", "-c", "if [ -f config.ru ]; then bundle exec rackup -p 8080 -o 0.0.0.0; elif [ -f app.rb ]; then ruby app.rb -p 8080 -o 0.0.0.0; else ruby main.rb; fi"]
`
}

func generateStaticDockerfile() string {
	return `FROM nginx:alpine
RUN printf 'server {\n    listen 8080;\n    server_name localhost;\n    location / {\n        root /usr/share/nginx/html;\n        index index.html index.htm;\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY . /usr/share/nginx/html
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
`
}

func generateFallbackDockerfile() string {
	return `FROM nginx:alpine
RUN printf 'server {\n    listen 8080;\n    server_name localhost;\n    location / {\n        root /usr/share/nginx/html;\n        autoindex on;\n        index index.html index.htm;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY . /usr/share/nginx/html
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
`
}
