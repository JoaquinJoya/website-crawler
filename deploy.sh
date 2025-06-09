#!/bin/bash

# Web Crawler Deployment Script for Hetzner
# Usage: ./deploy.sh [environment]
# Environment: dev (default) or prod

set -e

ENVIRONMENT=${1:-dev}
SERVER_IP=${SERVER_IP:-"your-server-ip"}
DOMAIN=${DOMAIN:-"crawler.yourdomain.com"}

echo "🚀 Deploying Web Crawler to Hetzner ($ENVIRONMENT environment)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null; then
    print_error "Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Build the Docker image
print_status "Building Docker image..."
docker build -t web-crawler:latest .

if [ $? -eq 0 ]; then
    print_success "Docker image built successfully"
else
    print_error "Failed to build Docker image"
    exit 1
fi

# Environment-specific deployment
if [ "$ENVIRONMENT" == "prod" ]; then
    print_status "Deploying to PRODUCTION environment"
    
    # Create production environment file
    cat > .env.prod << EOF
# Production Environment Configuration
SERVER_PORT=8081
GIN_MODE=release

# Cache Configuration
CACHE_DIR=/app/cache
BASELINE_DIR=/app/baselines

# Colly Configuration
COLLY_ENABLED=true
COLLY_USER_AGENT=Mozilla/5.0 (compatible; WebCrawler-Pro/1.0)
COLLY_DELAY=200ms
COLLY_RANDOM_DELAY=100ms
COLLY_PARALLELISM=5
COLLY_RESPECT_ROBOTS_TXT=true
COLLY_CACHE_ENABLED=true
COLLY_CACHE_TTL=24h

# Monitoring
MONITORING_MULTILANG_ENABLED=true
MONITORING_CHANGE_DETECTION=true

# Security
RATE_LIMIT_REQUESTS_PER_MINUTE=300
RATE_LIMIT_BURST=50

# Domain
DOMAIN=${DOMAIN}
EOF

    # Deploy with production profile (includes Nginx)
    print_status "Starting production deployment with Nginx reverse proxy..."
    docker-compose --profile production --env-file .env.prod up -d
    
    print_warning "Don't forget to:"
    echo "  1. Point your domain ${DOMAIN} to ${SERVER_IP}"
    echo "  2. Set up SSL certificates (Let's Encrypt recommended)"
    echo "  3. Configure firewall to allow ports 80 and 443"
    
else
    print_status "Deploying to DEVELOPMENT environment"
    
    # Create development environment file
    cat > .env.dev << EOF
# Development Environment Configuration
SERVER_PORT=8081
GIN_MODE=debug

# Cache Configuration
CACHE_DIR=/app/cache
BASELINE_DIR=/app/baselines

# Colly Configuration (more verbose for debugging)
COLLY_ENABLED=true
COLLY_USER_AGENT=Mozilla/5.0 (compatible; WebCrawler-Dev/1.0)
COLLY_DELAY=100ms
COLLY_RANDOM_DELAY=50ms
COLLY_PARALLELISM=3
COLLY_RESPECT_ROBOTS_TXT=true
COLLY_CACHE_ENABLED=true
COLLY_CACHE_TTL=1h
COLLY_DEBUG_MODE=true

# Monitoring
MONITORING_MULTILANG_ENABLED=true
MONITORING_CHANGE_DETECTION=true
EOF

    # Deploy development version (no Nginx)
    print_status "Starting development deployment..."
    docker-compose --env-file .env.dev up -d web-crawler
    
    print_warning "Development mode: Access directly via http://${SERVER_IP}:8081"
fi

# Wait for services to be ready
print_status "Waiting for services to start..."
sleep 10

# Health check
print_status "Performing health check..."
if [ "$ENVIRONMENT" == "prod" ]; then
    HEALTH_URL="http://localhost"
else
    HEALTH_URL="http://localhost:8081"
fi

for i in {1..10}; do
    if curl -f $HEALTH_URL/health &> /dev/null || curl -f $HEALTH_URL/ &> /dev/null; then
        print_success "✅ Web Crawler is running and healthy!"
        break
    else
        if [ $i -eq 10 ]; then
            print_error "❌ Health check failed after 10 attempts"
            print_status "Checking logs..."
            docker-compose logs web-crawler
            exit 1
        fi
        print_status "Health check attempt $i/10 failed, retrying in 3 seconds..."
        sleep 3
    fi
done

# Show running containers
print_status "Running containers:"
docker-compose ps

# Show access information
echo ""
print_success "🎉 Deployment completed successfully!"
echo ""
if [ "$ENVIRONMENT" == "prod" ]; then
    echo "🌐 Production Access:"
    echo "   URL: http://${DOMAIN}"
    echo "   Server IP: ${SERVER_IP}"
    echo ""
    echo "📋 Next Steps:"
    echo "   1. Configure DNS: ${DOMAIN} → ${SERVER_IP}"
    echo "   2. Set up SSL with Let's Encrypt"
    echo "   3. Configure team access authentication"
else
    echo "🛠️  Development Access:"
    echo "   URL: http://${SERVER_IP}:8081"
    echo ""
    echo "📋 Available Commands:"
    echo "   View logs: docker-compose logs -f web-crawler"
    echo "   Stop: docker-compose down"
    echo "   Restart: docker-compose restart web-crawler"
fi

echo ""
print_status "Resource Usage (CPX21 Server):"
echo "   CPU: ~2.5/3 vCPUs allocated"
echo "   RAM: ~3GB/4GB allocated"
echo "   Storage: Cache in /app/cache volume"
echo ""
print_warning "Monitor resource usage with: docker stats"