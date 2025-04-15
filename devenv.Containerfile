FROM debian:trixie as devenv

# Install claude-code for development
RUN apt-get update && apt-get install -y nodejs npm golang ca-certificates fish
RUN npm install -g @anthropic-ai/claude-code
