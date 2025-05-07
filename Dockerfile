FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

# Install required packages
RUN apt-get update && apt-get install -y \
    python3 \
    golang \
    g++ \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Create code directory
WORKDIR /code

# Keep container running
CMD ["tail", "-f", "/dev/null"]
