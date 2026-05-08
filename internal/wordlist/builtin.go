package wordlist

import (
	"fmt"
	"os"
	"strings"
)

// Words is the built-in top-1000 DNS bruteforce wordlist.
// These are the most commonly found subdomain prefixes across internet scans.
var Words = []string{
	// Core services
	"www", "mail", "ftp", "smtp", "pop", "pop3", "imap", "webmail", "remote",
	"vpn", "ns1", "ns2", "ns3", "ns4", "mx", "mx1", "mx2", "email", "api",
	"app", "web", "m", "mobile", "wap", "portal", "dashboard", "admin",
	"login", "secure", "cdn", "static", "assets", "media", "img", "images",
	"upload", "files", "docs", "support", "help", "status", "monitor",

	// Development / staging
	"dev", "dev1", "dev2", "dev3", "staging", "staging1", "stage", "uat",
	"qa", "test", "test1", "test2", "beta", "beta1", "alpha", "sandbox",
	"demo", "preview", "review", "pilot", "poc", "preprod", "prod",
	"production", "live", "old", "new", "temp", "backup", "v1", "v2",
	"v3", "v4", "release", "rc", "hotfix", "build", "ci", "cd",
	"int", "integration", "perf", "load", "stress", "feature", "experimental",

	// Web / app tiers
	"www1", "www2", "www3", "web1", "web2", "web3", "app1", "app2", "app3",
	"mobile1", "mobile2", "wap1", "api1", "api2", "api3",
	"portal1", "portal2", "dashboard1", "dashboard2",

	// Databases
	"db", "db1", "db2", "db3", "db4", "mysql", "postgres", "postgresql",
	"mongodb", "mongo", "redis", "elastic", "elasticsearch", "memcache",
	"memcached", "cassandra", "couchdb", "influx", "influxdb",
	"clickhouse", "mariadb", "mssql", "oracle", "aurora",

	// Cache / queue / messaging
	"cache", "cache1", "cache2", "queue", "mq", "kafka", "rabbit",
	"rabbitmq", "zookeeper", "nats", "activemq", "celery", "worker",
	"consumer", "producer", "broker", "pubsub", "bus",

	// Load balancing / proxy
	"lb", "lb1", "lb2", "lb3", "proxy", "proxy1", "proxy2",
	"reverse-proxy", "gateway", "api-gateway", "edge", "ingress",
	"haproxy", "nginx", "traefik", "envoy",

	// Infrastructure nodes
	"server", "server1", "server2", "server3", "server4",
	"host", "host1", "host2", "host3", "node", "node1", "node2",

	// Container / orchestration
	"docker", "k8s", "kubernetes", "kube", "helm", "rancher",
	"portainer", "harbor", "registry", "gcr", "ecr", "acr",

	// CI/CD
	"jenkins", "jenkins1", "ci1", "ci2", "cd1",
	"gitlab", "gitea", "gitea1", "git", "svn",
	"teamcity", "bamboo", "argocd", "flux", "drone",
	"buildkite", "circleci", "travis",

	// Source control / repos
	"repo", "repos", "repository", "artifact", "artifacts",
	"nexus", "nexus2", "sonar", "sonarqube", "artifactory",

	// Authentication / identity
	"auth", "sso", "id", "identity", "account", "accounts",
	"profile", "profiles", "user", "users", "oauth", "oidc",
	"saml", "keycloak", "ldap", "ad", "radius", "kerberos",
	"mfa", "2fa", "signup", "register", "password", "reset",
	"verify", "confirm", "token", "session",

	// API variants
	"apiv1", "apiv2", "apiv3", "rest", "graphql", "soap",
	"rpc", "grpc", "webhook", "webhooks", "callback",
	"v1", "v2", "v3", "service", "services", "ws",
	"websocket", "stream", "streaming", "push", "notification",
	"notifications", "event", "events", "message", "messages",

	// Business applications
	"crm", "erp", "hr", "payroll", "finance", "accounting",
	"legal", "compliance", "marketing", "sales", "purchase",
	"procurement", "inventory", "warehouse", "supply", "logistics",
	"shipping", "delivery", "tracking", "order", "orders",
	"cart", "checkout", "payment", "payments", "billing",
	"invoice", "invoices", "subscription", "pricing", "quote",
	"contract", "contracts", "ticket", "tickets", "project",
	"projects", "task", "tasks", "issue", "issues",
	"request", "requests", "case", "cases",

	// Internal tools
	"intranet", "extranet", "internal", "corp", "corporate",
	"private", "vpn1", "vpn2", "rdp", "citrix", "terminal",
	"owa", "exchange", "sharepoint", "outlook", "teams",
	"confluence", "jira", "jira1", "wiki", "kb",
	"helpdesk", "servicedesk", "itsm", "cmdb",
	"monitoring", "nagios", "zabbix", "icinga",
	"grafana", "kibana", "prometheus", "alertmanager",
	"pagerduty", "opsgenie", "datadog", "newrelic",
	"sentry", "splunk", "elk", "logstash",

	// Content / media / publishing
	"blog", "news", "press", "photos", "gallery",
	"video", "videos", "podcast", "feed", "rss", "sitemap",
	"download", "downloads", "documentation", "forum",
	"community", "events", "calendar", "newsletter",

	// E-commerce
	"shop", "store", "ecom", "ecommerce", "marketplace",
	"catalog", "product", "products", "category", "search",
	"wishlist", "compare", "review", "reviews", "rating",
	"coupon", "coupon1", "promo", "promotion", "discount",
	"affiliate", "affiliates", "reseller", "partner", "partners",

	// Security
	"security", "sec", "waf", "firewall", "ids", "ips",
	"siem", "soc", "pentest", "scan", "scanner",
	"vault", "secrets", "cert", "certificate", "ca", "pki",
	"key", "keys", "pgp", "gpg", "encrypt", "decrypt",
	"hash", "sign",

	// Cloud provider specific
	"cloud", "cloud1", "cloud2", "aws", "azure", "gcp",
	"s3", "ec2", "elb", "rds", "lambda", "cloudfront",
	"fastly", "akamai", "cloudflare", "heroku",
	"digitalocean", "linode", "vultr", "ovh", "hetzner",

	// Observability / logging
	"logs", "log", "logging", "metrics", "metric",
	"trace", "tracing", "telemetry", "observability",
	"analytics", "reporting", "bi", "data", "stats",
	"dashboard2", "reports",

	// Mail infrastructure
	"smtp1", "smtp2", "pop4", "imap2", "webmail2",
	"mail1", "mail2", "mail3", "mail4", "email2",
	"autoresponder", "bounce", "spam", "antispam",
	"filter", "quarantine", "archive",

	// DNS / network infra
	"dns", "dns1", "dns2", "nameserver", "ns5", "ns6",
	"resolv", "resolver", "relay", "mx3", "mx4",
	"ntp", "snmp", "syslog", "netflow", "ipam",
	"dhcp", "tacacs", "bgp", "ospf", "mpls",

	// Disaster recovery / HA
	"dr", "disaster", "recovery", "failover", "standby",
	"replica", "slave", "master", "primary", "secondary",
	"cluster", "cluster1", "cluster2", "ha", "hot",
	"warm", "cold", "offsite",

	// Geographic / regional
	"us", "eu", "uk", "au", "ca", "ap", "sg", "jp",
	"br", "mx", "in", "de", "fr", "it", "es", "nl",
	"ru", "cn", "us1", "us2", "eu1", "eu2", "ap1", "ap2",
	"asia", "europe", "america", "latam", "na",
	"global", "worldwide", "international",

	// SSH / file transfer
	"sftp", "sftp1", "ftp1", "ftp2", "ftps",
	"ssh", "ssh1", "bastion", "jump", "jumpbox",

	// Misc numbered variants
	"api4", "api5", "app4", "app5", "web4", "web5",
	"dev4", "dev5", "test3", "test4", "staging2",
	"old2", "backup2", "backup3", "temp2",
	"server5", "server6", "host4", "node3",
	"db5", "db6", "cache3", "lb4", "proxy3",

	// Common paths as subdomains
	"static1", "static2", "assets1", "assets2",
	"media1", "media2", "files1", "files2",
	"upload1", "upload2", "img1", "img2",
	"cdn1", "cdn2", "cdn3",

	// SaaS-style
	"app-dev", "app-test", "app-staging", "app-prod",
	"api-dev", "api-test", "api-staging", "api-prod",
	"web-dev", "web-test", "web-staging", "web-prod",

	// Subweb patterns
	"my", "my2", "myaccount", "myapp", "myweb",
	"client", "clients", "customer", "customers",
	"vendor", "vendors", "agent", "agents", "dealer",

	// Management interfaces
	"mgmt", "manage", "management", "control", "panel",
	"controlpanel", "cp", "wcp", "whm", "cpanel",
	"plesk", "webmin", "pma", "phpmyadmin", "adminer",

	// Search / discovery
	"solr", "sphinx", "algolia", "meilisearch",
	"opensearch", "search1", "search2",

	// Object storage
	"storage", "storage1", "storage2", "blob",
	"bucket", "object", "s3like", "minio",

	// VoIP / communication
	"sip", "voip", "pbx", "asterisk", "freeswitch",
	"xmpp", "jabber", "irc", "chat", "chat1", "chat2",
	"meet", "conference", "zoom", "webex", "video1",

	// Analytics
	"track", "tracking", "pixel", "tag", "tagmanager",
	"gtm", "matomo", "piwik", "hotjar", "mixpanel",
	"segment", "heap", "amplitude", "optimizely",

	// Misc enterprise
	"erp1", "sap", "oracle1", "siebel", "salesforce",
	"hubspot", "marketo", "pardot", "mailchimp",
	"sendgrid", "twilio", "stripe", "braintree", "paypal",

	// Numeric patterns
	"1", "2", "3", "4", "5", "10", "100",

	// Commonly found in the wild (misc)
	"info", "home", "public", "private1", "shared",
	"common", "util", "utils", "tools", "lab", "labs",
	"research", "r-and-d", "innovation", "incubator",
	"prototype", "mvp", "spike", "hack", "hackathon",
	"devhub", "hub", "platform", "platform1",
	"marketplace1", "exchange1", "exchange2",
	"connect", "connector", "integration1",
	"sync", "async", "batch", "job", "jobs",
	"cron", "scheduler", "etl", "pipeline",
	"import", "export", "migrate", "migration",
	"onboard", "onboarding", "setup", "install",
	"activate", "activation", "welcome",
	"healthcheck", "health", "ping", "alive",
	"readiness", "liveness", "ready",

	// High-value bug bounty targets (frequently found exposed)
	"secret", "secrets2", "credentials", "creds", "token", "tokens",
	"env", ".env", "config", "configs", "configuration",
	"settings", "setting", "properties", "props",
	"passwd", "password", "passwords", "shadow",
	"htpasswd", "htaccess",

	// Internal / corporate patterns seen in BB programs
	"internal2", "int1", "int2", "corp1", "corp2",
	"extranet1", "partner1", "partner2", "b2b", "b2c",
	"wholesale", "franchise",
	"hr1", "hr2", "payroll1", "finance", "finance1",
	"legal", "compliance", "audit", "risk",
	"operations", "ops", "ops1", "ops2", "noc",
	"helpdesk1", "itsupport", "it",

	// Dev tooling / SDLC
	"sonar", "sonarqube", "codecov", "coveralls",
	"terraform", "ansible", "chef", "puppet",
	"artifactory", "nexus", "registry", "harbor",
	"argocd", "flux", "spinnaker", "octopus",
	"teamcity", "bamboo", "travis", "circleci",
	"drone", "concourse",

	// Kubernetes / container
	"k8s", "kube", "kubernetes", "rancher",
	"cluster3", "pod", "node4", "node5",
	"docker", "container", "registry1",

	// Auth / identity providers
	"ldap", "ad", "activedirectory", "saml",
	"keycloak", "okta", "auth0", "ping",
	"iam", "idp", "sso1", "sso2",
	"mfa", "2fa", "otp", "totp",

	// API versioning patterns
	"api-v3", "api-v4", "api-gateway", "gateway1",
	"apigee", "kong", "tyk",
	"graphql", "rest", "grpc", "websocket1",
	"webhook1", "webhooks1",

	// Mobile / app backends
	"android", "ios", "native", "hybrid",
	"push", "notification", "notifications",
	"fcm", "apns", "onesignal",

	// Payment / fintech
	"payments", "payment1", "payment2",
	"billing1", "billing2", "invoice1",
	"checkout", "cart", "orders",
	"refund", "refunds", "dispute",
	"fraud", "risk1", "kyc", "aml",

	// CDN / edge
	"edge", "edge1", "edge2",
	"origin", "origin1", "origin2",
	"cache1", "cache2", "cache4",
	"purge", "invalidate",

	// Regional datacenters / AZs
	"us-east", "us-west", "eu-west", "eu-central",
	"ap-southeast", "ap-northeast",
	"use1", "usw2", "euw1", "apse1",

	// Numbered servers (higher range seen in large orgs)
	"server7", "server8", "server9", "server10",
	"host5", "host6", "host7", "host8",
	"node6", "node7", "node8", "node9",
	"db7", "db8", "db9", "db10",

	// Misc found frequently in cert transparency / crawls
	"go", "ruby", "python", "java", "php", "node",
	"service1", "service2", "service3",
	"micro", "micro1", "micro2",
	"worker1", "worker2",
	"broker", "broker1", "queue1",
	"event", "events", "eventbus",
	"pubsub", "kafka1", "rabbit", "nats",

	// Final top-1000 fill — high-frequency in CT logs and passive recon
	"account", "accounts", "user", "users", "member", "members",
	"profile", "profiles", "feed1", "stream",
	"notify", "alert", "alerts", "webhook2",
	"report1", "report2", "insight", "insights",
	"workspace", "workspaces", "project", "projects",
	"team", "teams", "org", "orgs", "organization",
	"invite", "invites", "ref", "referral",
	"token1", "session", "sessions", "cookie",
	"proxy1", "proxy2", "lb1", "lb2", "lb3",
	"fw", "fw1", "utm", "utm1",
}

// WriteToFile writes the built-in wordlist to path, one word per line.
// Returns the number of words written.
func WriteToFile(path string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create wordlist file: %w", err)
	}
	defer f.Close()

	content := strings.Join(Words, "\n") + "\n"
	if _, err := f.WriteString(content); err != nil {
		return 0, fmt.Errorf("write wordlist: %w", err)
	}
	return len(Words), nil
}
