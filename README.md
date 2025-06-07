# Website Crawler with Advanced Monitoring

🚀 **Professional-grade web crawler** built with Go and Colly framework, featuring intelligent caching, multi-language monitoring, and content change detection.

## ✨ Features

### 🕷️ **Advanced Web Crawling**
- **Sophisticated URL discovery** with 100+ CSS selectors
- **Webflow-specific patterns** for hidden dropdown navigation
- **Language-aware crawling** (EN/ES) with automatic detection
- **Smart content extraction** (titles, meta descriptions, headings, etc.)
- **Built with Colly framework** for reliability and performance

### 💾 **Intelligent Caching System**
- Automatic page caching with configurable TTL (24h default)
- Cache hit rate tracking and performance monitoring
- Automatic cleanup of expired cache entries
- Massive speed improvements on re-crawls

### 🌍 **Multi-Language Monitoring**
- Automatic language detection from URLs and HTML
- Synchronization analysis between English/Spanish versions
- Missing translation detection with actionable alerts
- Content length comparison and quality analysis
- Untranslated content identification

### 🚨 **Content Change Detection**
- Smart baseline creation for all crawled pages
- Advanced content comparison using similarity algorithms
- Change categorization: new, modified, deleted, title_changed
- Severity scoring: low, medium, high, critical
- Webhook alerts for important content changes
- Word count delta tracking and structure analysis

## 🎯 Perfect for Multilingual Websites

Originally built for **hisonrisa dental practice** to monitor their bilingual (English/Spanish) website, ensuring content synchronization and detecting changes across both language versions.

## 🚀 Quick Start

### Prerequisites
- Go 1.19+ installed
- Python 3.8+ for AI processing
- Git for version control

### AI Provider Requirements
- **OpenAI**: `pip install openai`
- **Claude**: `pip install anthropic`  
- **Gemini**: `pip install google-genai` (new API)

### Installation
```bash
git clone https://github.com/JoaquinJoya/website-crawler.git
cd website-crawler
go mod tidy
go build -o web-crawler
```

### Basic Usage
```bash
# Start the server
./web-crawler

# Crawl a website
curl -X POST "http://localhost:8081/crawl" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://your-website.com"}'

# Check monitoring statistics
curl "http://localhost:8081/monitoring/stats"

# Analyze language synchronization
curl "http://localhost:8081/monitoring/language-sync?url=https://your-website.com"
```

## 📊 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web interface |
| `/crawl` | POST | Crawl a website and return results |
| `/stream-crawl` | GET | Real-time streaming crawl |
| `/monitoring/stats` | GET | Comprehensive monitoring statistics |
| `/monitoring/language-sync` | GET | Multi-language synchronization analysis |

## ⚙️ Configuration

Configure via environment variables:

```bash
# Server settings
export PORT=8081

# Colly framework settings
export COLLY_ENABLED=true
export COLLY_PARALLELISM=5
export COLLY_DELAY=200ms
export COLLY_RANDOM_DELAY=100ms

# Caching settings
export COLLY_CACHE_ENABLED=true
export COLLY_CACHE_TTL=24h
export COLLY_CACHE_DIR=./cache

# Monitoring settings
export MONITORING_MULTILANG=true
export MONITORING_CHANGE_DETECTION=true
export MONITORING_WEBHOOK_URL="https://hooks.slack.com/your-webhook"
export MONITORING_THRESHOLD=0.8
```

## 🏗️ Architecture

```
├── main.go                 # HTTP server and routing
├── config/                 # Configuration management
├── crawler/                # Web crawling logic
│   ├── discovery.go        # URL discovery algorithms
│   ├── fetcher.go         # Page fetching interface
│   ├── colly_crawler.go   # Colly-based crawler
│   └── colly_fetcher.go   # Colly-based fetcher
├── monitoring/             # Advanced monitoring features
│   ├── cache.go           # Intelligent caching system
│   ├── multilang.go       # Multi-language monitoring
│   ├── changes.go         # Content change detection
│   └── monitor.go         # Integrated monitoring
├── extract/                # Content extraction utilities
└── ai/                     # AI processing integration
```

## 🛠️ Technology Stack

- **Go** - High-performance backend language
- **Colly** - Professional web scraping framework
- **Gin** - Fast HTTP web framework
- **goquery** - jQuery-like HTML parsing
- **Custom monitoring system** - Advanced tracking and alerting

## 🌟 Advanced Features

### Smart URL Discovery
- 100+ CSS selectors for comprehensive link discovery
- Webflow-specific dropdown and navigation detection
- Language alternate link detection (hreflang)
- XML sitemap parsing
- JavaScript URL extraction
- Smart pattern generation for language URLs

### Professional Monitoring
- Cache performance metrics
- Language coverage analysis
- Content change tracking with similarity scoring
- Webhook integration for real-time alerts
- Detailed reporting and statistics

## 🎯 Use Cases

- **Multilingual website monitoring** - Ensure content synchronization
- **SEO monitoring** - Track title and meta description changes
- **Content management** - Monitor updates and translations
- **Performance optimization** - Leverage intelligent caching
- **Competitive analysis** - Monitor competitor websites
- **Website auditing** - Comprehensive site analysis

## 📈 Performance

- **Intelligent caching** reduces repeat crawl times by 90%
- **Concurrent processing** with configurable parallelism
- **Rate limiting** respects target websites
- **Automatic retries** ensure reliability
- **Memory efficient** streaming for large sites

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with ❤️ for **hisonrisa dental practice**
- Powered by the excellent [Colly framework](https://github.com/gocolly/colly)
- Inspired by the need for professional multilingual website monitoring

---

**Made with 🦷 for dental professionals who need reliable website monitoring**