# 🚀 Hetzner Deployment Guide for Web Crawler

Complete guide to deploy your web crawler on Hetzner Cloud optimized for Latin America.

## 🌎 Recommended Server Configuration

### **Server Choice: CPX21**
- **Location:** Ashburn, VA (US-East) - Best latency for Latin America
- **Specs:** 3 vCPUs, 4GB RAM, 80GB SSD, 20TB traffic
- **Cost:** ~€4.90/month
- **Perfect for:** Team of 8, crawling 19-600 pages per session

### **Why CPX21 is Optimal:**
✅ **CPU:** 3 vCPUs handle concurrent crawling + AI processing  
✅ **Memory:** 4GB sufficient for caching + Docker overhead  
✅ **Storage:** 80GB plenty for cache and logs  
✅ **Traffic:** 20TB covers extensive crawling needs  
✅ **Latency:** 80-150ms to Latin America (excellent)

## 🛠️ Deployment Steps

### 1. Create Hetzner Server

1. **Log into Hetzner Cloud Console**
2. **Create new server:**
   - **Image:** Ubuntu 22.04 LTS
   - **Type:** CPX21 (3 vCPUs, 4GB RAM)
   - **Location:** Ashburn, VA
   - **Networking:** IPv4 + IPv6
   - **SSH Key:** Upload your public key
   - **Firewall:** Create firewall with rules:
     ```
     HTTP (80)    - 0.0.0.0/0, ::/0
     HTTPS (443)  - 0.0.0.0/0, ::/0  
     SSH (22)     - Your IP only
     Custom 8081  - 0.0.0.0/0, ::/0 (for direct access)
     ```

### 2. Server Setup

SSH into your server and run:

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Logout and login again for Docker group to take effect
exit
```

### 3. Deploy Application

```bash
# Clone your repository
git clone https://github.com/JoaquinJoya/website-crawler.git
cd website-crawler

# Deploy (choose environment)
./deploy.sh dev   # Development (direct access on port 8081)
./deploy.sh prod  # Production (with Nginx reverse proxy)
```

## 🔧 Configuration Options

### Development Deployment
- **Access:** `http://YOUR_SERVER_IP:8081`
- **Features:** Debug mode, verbose logging
- **Use case:** Testing and development

### Production Deployment
- **Access:** `http://YOUR_DOMAIN` (port 80)
- **Features:** Nginx reverse proxy, rate limiting, security headers
- **Use case:** Team production usage

## 🌐 Domain Setup (Production)

1. **Point your domain to server IP:**
   ```
   A Record: crawler.yourdomain.com → YOUR_SERVER_IP
   ```

2. **Set up SSL (recommended):**
   ```bash
   # Install Certbot
   sudo apt install certbot python3-certbot-nginx
   
   # Get SSL certificate
   sudo certbot --nginx -d crawler.yourdomain.com
   ```

## 📊 Resource Monitoring

### Monitor resource usage:
```bash
# Container resource usage
docker stats

# System resource usage
htop

# Disk usage
df -h

# Log sizes
du -sh /var/lib/docker/volumes/website-crawler_*
```

### Expected Usage (Team of 8):
- **CPU:** 20-40% average, 70-90% during large crawls
- **RAM:** 1-2GB normal, 2.5-3GB during intensive crawling
- **Storage:** 500MB-2GB for cache (auto-managed)
- **Network:** 10-100MB per crawl session

## 🔒 Security Recommendations

### 1. Team Access Control
```bash
# Create team user accounts (optional)
sudo adduser teammate1
sudo usermod -aG docker teammate1

# Or use basic auth in Nginx (uncomment in nginx.conf)
sudo htpasswd -c /etc/nginx/.htpasswd team
```

### 2. Firewall Rules
```bash
# UFW firewall setup
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow 80
sudo ufw allow 443
sudo ufw allow 8081  # If using development mode
sudo ufw enable
```

### 3. Regular Updates
```bash
# Update system monthly
sudo apt update && sudo apt upgrade -y

# Update Docker images weekly
cd website-crawler
docker-compose pull
docker-compose up -d
```

## 🔄 Management Commands

```bash
# View logs
docker-compose logs -f web-crawler

# Restart application
docker-compose restart web-crawler

# Stop everything
docker-compose down

# Update application
git pull origin main
docker-compose build
docker-compose up -d

# Clean up old images/volumes
docker system prune -a
```

## 💰 Cost Estimation

### Monthly Costs (CPX21):
- **Server:** €4.90/month
- **Traffic:** Included (20TB)
- **Backups:** €0.98/month (optional, recommended)
- **Total:** ~€6/month

### Annual Cost: ~€72/year

## 🆘 Troubleshooting

### Application won't start:
```bash
docker-compose logs web-crawler
```

### High memory usage:
```bash
# Adjust resource limits in docker-compose.yml
docker-compose restart web-crawler
```

### SSL certificate issues:
```bash
sudo certbot renew --dry-run
```

### Cache filling up disk:
```bash
# Clear cache
docker-compose exec web-crawler rm -rf /app/cache/*
docker-compose restart web-crawler
```

## 📞 Support

- **Hetzner Support:** https://docs.hetzner.com/
- **Docker Issues:** Check logs with `docker-compose logs`
- **Application Issues:** Monitor via `/monitoring` endpoint

---

**Perfect setup for your Latin American team! 🌎✨**