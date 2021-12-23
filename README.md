# alya-go-fn-boilerplate 
Go-fn Boilerplate for Web Functions  

This projects aiming to create a deployment ready go skeleton webserver for fast development uses Gin.  

# Usage  
Usage is simple! Basically Clone->Run-Requirements->Build->Run-App consept.  

- git clone https://git.yazgan.xyz/alperreha/alya-go-fn-boilerplate
- cd alya-go-fn-boilerplate
- docker-compose up -d // create and run require containers (NATS,Redis, Minio and Postgresql)  
- docker build -t postapp . (e.g. docker build -t hub.yazgan.xyz/myapp-go-service:1.0.0 . )  
- Copy .env-test to .env file and configure your own. (e.g. `cp .env-test .env`)  
- docker run --name alyafnpost -p 9090:9090

# TODO:
TODO: 
[X] - Validators -> (Gin and github.com/go-playground/validator/v10)  
[X] - ORM -> Gorm (Supports Sqlite, Postgres and SQL Databases)  
[X] - NATS Event Pub&Sub, Request&Reply  
[X] - Health Check Probe (GET /app_kernel_stats with basic auth in .env file)  
[-] - Rate Limiting  



