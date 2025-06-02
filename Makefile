# Redigo Streams Makefile

.PHONY: help build test clean proto install-tools example redis-start redis-stop docker-redis docker-dev docker-build docker-clean

# Default target
help:
	@echo "Available commands:"
	@echo "  build              - Build the project"
	@echo "  test               - Run tests"
	@echo "  proto              - Generate protobuf files"
	@echo "  install-tools      - Install required tools"
	@echo ""
	@echo "  🐳 Docker Commands:"
	@echo "  docker-redis       - Start Redis with Docker Compose"
	@echo "  docker-dev         - Start development environment"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-clean       - Clean Docker resources"
	@echo ""
	@echo "  📋 Examples (Producer/Consumer Split):"
	@echo "  producer-basic     - Run basic producer"
	@echo "  consumer-basic     - Run basic consumer"
	@echo "  producer-dedup     - Run deduplication producer"
	@echo "  consumer-dedup     - Run deduplication consumer"
	@echo "  producer-delayed   - Run delayed task producer"
	@echo "  consumer-delayed   - Run delayed task consumer"
	@echo "  producer-recovery  - Run recovery producer"
	@echo "  consumer-recovery  - Run recovery consumer"
	@echo "  producer-concurrent - Run concurrent producer"
	@echo "  consumer-concurrent - Run concurrent consumer"
	@echo "  producer-multi     - Run multi-consumer producer"
	@echo "  consumer-multi     - Run multi-consumer safety test"
	@echo ""
	@echo "  📋 Legacy Examples (Combined):"
	@echo "  example-basic      - Run basic example"
	@echo "  example-recovery   - Run recovery example"
	@echo "  example-concurrent - Run concurrent example"
	@echo "  example-multi-consumer - Run multi-consumer example"
	@echo "  example-delayed    - Run delayed task example"
	@echo "  example-deduplication - Run deduplication example"
	@echo ""
	@echo "  🛠️  Maintenance:"
	@echo "  redis-start        - Start Redis server (Docker)"
	@echo "  redis-stop         - Stop Redis server (Docker)"
	@echo "  clean              - Clean build artifacts"

# Install required tools
install-tools:
	@echo "Installing protobuf compiler..."
	@which protoc || (echo "Please install protobuf compiler: brew install protobuf" && exit 1)
	@echo "Installing Go protobuf plugin..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative pkg/proto/message.proto

# Build the project
build: proto
	@echo "Building project..."
	go mod tidy
	go build ./...

# Run tests
test: build
	@echo "Running tests..."
	go test -v ./...

# =============================================================================
# 🐳 DOCKER COMMANDS
# =============================================================================

# Start Redis with Docker Compose
docker-redis:
	@echo "🐳 Starting Redis with Docker Compose..."
	docker-compose up -d redis
	@echo "✅ Redis available at localhost:6379"
	@echo "💡 Use 'docker-compose logs -f redis' to see logs"

# Start development environment (Redis + Go shell)
docker-dev:
	@echo "🐳 Starting development environment..."
	docker-compose up -d redis dev
	@echo "✅ Development environment ready!"
	@echo "💡 Use 'docker-compose exec dev sh' to access Go shell"
	@echo "💡 Redis is available at redis:6379 from within containers"

# Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t redigo-streams .

# Clean Docker resources
docker-clean:
	@echo "🐳 Cleaning Docker resources..."
	docker-compose down -v
	@echo "✅ Docker resources cleaned"

# Stop all Docker services
docker-stop:
	@echo "🐳 Stopping all Docker services..."
	docker-compose down

# =============================================================================
# 📋 SPLIT EXAMPLES (Producer/Consumer Separate)
# =============================================================================

# Basic Examples
producer-basic: docker-redis
	@echo "📤 Running basic producer..."
	@echo "💡 Run 'make consumer-basic' in another terminal to see the flow"
	@sleep 2
	cd examples/basic/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-basic: docker-redis
	@echo "🎯 Running basic consumer..."
	@echo "💡 Run 'make producer-basic' in another terminal to send messages"
	@sleep 2
	cd examples/basic/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Deduplication Examples
producer-dedup: docker-redis
	@echo "📤 Running deduplication producer..."
	@echo "💡 Run 'make consumer-dedup' in another terminal to see deduplication in action"
	@sleep 2
	cd examples/deduplication/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-dedup: docker-redis
	@echo "🎯 Running deduplication consumer..."
	@echo "💡 Run 'make producer-dedup' in another terminal to test deduplication"
	@sleep 2
	cd examples/deduplication/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Delayed Task Examples
producer-delayed: docker-redis
	@echo "📅 Running delayed task producer..."
	@echo "💡 Run 'make consumer-delayed' in another terminal to process scheduled tasks"
	@sleep 2
	cd examples/delayed/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-delayed: docker-redis
	@echo "⏰ Running delayed task consumer..."
	@echo "💡 Run 'make producer-delayed' in another terminal to schedule tasks"
	@sleep 2
	cd examples/delayed/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Recovery Examples
producer-recovery: docker-redis
	@echo "🔄 Running recovery producer..."
	@echo "💡 Run 'make consumer-recovery' in another terminal to test message recovery"
	@sleep 2
	cd examples/recovery/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-recovery: docker-redis
	@echo "🔄 Running recovery consumer..."
	@echo "💡 Run 'make producer-recovery' in another terminal to send test messages"
	@sleep 2
	cd examples/recovery/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Concurrent Processing Examples
producer-concurrent: docker-redis
	@echo "🔧 Running concurrent producer..."
	@echo "💡 Run 'make consumer-concurrent' in another terminal to see concurrent processing"
	@sleep 2
	cd examples/concurrent/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-concurrent: docker-redis
	@echo "🔧 Running concurrent consumer..."
	@echo "💡 Run 'make producer-concurrent' in another terminal to send high-load messages"
	@sleep 2
	cd examples/concurrent/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Multi-Consumer Safety Examples
producer-multi: docker-redis
	@echo "👥 Running multi-consumer producer..."
	@echo "💡 Run 'make consumer-multi' in another terminal to test consumer safety"
	@sleep 2
	cd examples/multi-consumer/producer && REDIS_URL=redis://localhost:6379 go run main.go

consumer-multi: docker-redis
	@echo "👥 Running multi-consumer safety test..."
	@echo "💡 Run 'make producer-multi' in another terminal to send test messages"
	@sleep 2
	cd examples/multi-consumer/consumer && REDIS_URL=redis://localhost:6379 go run main.go

# =============================================================================
# 📋 LEGACY EXAMPLES (Combined)
# =============================================================================

# Run basic example (with Docker Redis)
example-basic: docker-redis
	@echo "📋 Running basic example..."
	@sleep 2
	cd examples/basic && REDIS_URL=redis://localhost:6379 go run main.go

# Run recovery example (with Docker Redis)
example-recovery: docker-redis
	@echo "📋 Running recovery example..."
	@sleep 2
	cd examples/recovery && REDIS_URL=redis://localhost:6379 go run main.go

# Run concurrent example (with Docker Redis)
example-concurrent: docker-redis
	@echo "📋 Running concurrent example..."
	@sleep 2
	cd examples/concurrent && REDIS_URL=redis://localhost:6379 go run main.go

# Run multi-consumer example (with Docker Redis)
example-multi-consumer: docker-redis
	@echo "📋 Running multi-consumer example..."
	@sleep 2
	cd examples/multi-consumer && REDIS_URL=redis://localhost:6379 go run main.go

# Run delayed task example (with Docker Redis)
example-delayed: docker-redis
	@echo "📋 Running delayed task example..."
	@sleep 2
	cd examples/delayed && REDIS_URL=redis://localhost:6379 go run main.go

# Run deduplication example (with Docker Redis)
example-deduplication: docker-redis
	@echo "📋 Running deduplication example..."
	@sleep 2
	cd examples/deduplication && REDIS_URL=redis://localhost:6379 go run main.go

# =============================================================================
# 🛠️  LEGACY COMMANDS (for backward compatibility)
# =============================================================================

# Legacy Redis commands (now use Docker)
redis-start: docker-redis
	@echo "ℹ️  Legacy command. Using Docker Compose instead."

redis-stop: docker-stop
	@echo "ℹ️  Legacy command. Using Docker Compose instead."

# Run basic example (legacy)
example: example-basic
	@echo "ℹ️  Legacy command. Use 'make example-basic' instead."

# Clean build artifacts
clean:
	@echo "Cleaning..."
	go clean ./...
	rm -f pkg/proto/*.pb.go

# =============================================================================
# 🚀 QUICK START
# =============================================================================

# Complete development setup
dev-setup: install-tools proto docker-redis
	@echo "🚀 Development environment ready!"
	@echo ""
	@echo "🎬 Producer/Consumer Split Commands:"
	@echo "  Terminal 1: make consumer-basic     # Start consumer first"
	@echo "  Terminal 2: make producer-basic     # Start producer"
	@echo ""
	@echo "  Terminal 1: make consumer-dedup     # Start dedup consumer"
	@echo "  Terminal 2: make producer-dedup     # Test deduplication"
	@echo ""
	@echo "Redis is running at localhost:6379"

# Quick demo with split terminals
demo-split:
	@echo "🎬 Split Demo Instructions:"
	@echo ""
	@echo "🔰 Basic Flow:"
	@echo "   Terminal 1: make consumer-basic"
	@echo "   Terminal 2: make producer-basic"
	@echo ""
	@echo "🔄 Deduplication Test:"
	@echo "   Terminal 1: make consumer-dedup"
	@echo "   Terminal 2: make producer-dedup"
	@echo ""
	@echo "⏰ Delayed Tasks:"
	@echo "   Terminal 1: make consumer-delayed"
	@echo "   Terminal 2: make producer-delayed"
	@echo ""
	@echo "🔄 Message Recovery:"
	@echo "   Terminal 1: make consumer-recovery"
	@echo "   Terminal 2: make producer-recovery"
	@echo ""
	@echo "🔧 Concurrent Processing:"
	@echo "   Terminal 1: make consumer-concurrent"
	@echo "   Terminal 2: make producer-concurrent"
	@echo ""
	@echo "👥 Multi-Consumer Safety:"
	@echo "   Terminal 1: make consumer-multi"
	@echo "   Terminal 2: make producer-multi"
	@echo ""
	@echo "💡 Watch the message flow between producer and consumer!" 