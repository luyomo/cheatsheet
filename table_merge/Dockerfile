FROM ubuntu:22.04

WORKDIR /app

# Install dependencies (GLIBC 2.34+ comes with the base image)
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy the binary and required folders
COPY bin/ ./bin/

RUN chmod +x ./bin/dm-toolkit

# ENTRYPOINT allows you to append arguments at runtime
ENTRYPOINT ["./bin/dm-toolkit"]
