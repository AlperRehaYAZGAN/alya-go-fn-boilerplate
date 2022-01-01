/**
*	Author: Alper Reha Yazgan
*	Date: 2021-12-18
*	Description: Go Gin Boilerplate
*
*	Main function creates main app scaffold and for every endpoint
*	use this procedure:
*	1. Create seperate handler function (e.g. getSuppliersHandler)
*	2. Validate request and cast it to dto (e.g. CreateSupplierDto, PostSupplierDtoValidator(), etc.)
*	3. Connect to database (e.g. ConnectDatabase)
*	4. Do your database operations (e.g. db.Create(&supplier))
*	5. Emit event for notify other services for changes (e.g. emitEvent)
*	6. Return response;
 */
package main

// @host localhost:8086
// @BasePath /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.basic BasicAuth
// @in header
// @name Authentication

// @title KampusApp Server
// @version 1.0
// @description YTU Kampusapp Server

// @contact.name Alya API Support
// @contact.url https://git.yazgan.xyz/alperreha/
// @contact.email support@alperreha.yazgan.xyz

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

import (
	// system packages
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// third party packages
	"git.yazgan.xyz/alperreha/kampusapp-final/docs"
	osstatus "github.com/fukata/golang-stats-api-handler"
	"github.com/joho/godotenv"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// web server packages
	"github.com/gin-gonic/gin"
	// page cacher
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"

	// security headers
	"github.com/gin-contrib/secure"
	// rbac middleware

	// validator packages
	"github.com/go-playground/validator/v10"
	// database packages
	// "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// event packages
	// go get github.com/nats-io/nats.go/@v1.13.0
	"github.com/nats-io/nats.go"
)

/**
*	ConnectNats : Connect to Nats
 */
var nc *nats.Conn

func InitNatsConnection() (*nats.Conn, error) {
	// get nats url from .env file like NATS_URL=nats://localhost:4222
	natsUrl := os.Getenv("NATS_URL")
	if natsUrl == "" {
		natsUrl = "nats://localhost:4222"
	}
	// connect to nats
	nc, err := nats.Connect(natsUrl)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

/**
*	Database Pool Variable
 */
var db *gorm.DB

func InitDbConnection(dbConnString string) {
	var err error
	//sqlite
	db, err = gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	// postgres
	// db, err = gorm.Open(postgres.Open(dbConnString), &gorm.Config{})
	if err != nil {
		log.Panic(err)
	}
}

// User object for Gorm
type User struct {
	gorm.Model
	Body            string     `gorm:"column:body;size:255;not null" json:"body" validate:"required,min=1,max=255"`
	Username        string     `gorm:"column:username;size:32;not null" json:"username" validate:"required,min=1,max=32"`
	Nickname        string     `gorm:"column:nickname;size:16;not null" json:"nickname" validate:"required,min=1,max=16"`
	Slug            string     `gorm:"column:slug;size:16;not null" json:"slug" validate:"required,min=1,max=16"`
	Email           string     `gorm:"column:email;size:255;not null" json:"email" validate:"required,min=1,max=255"`
	Password        string     `gorm:"column:password;size:128;not null" json:"password" validate:"required,min=1,max=128"`
	Type            int        `gorm:"column:type;not null;default:0" json:"type" validate:"required,min=1,max=4"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at;default:null" json:"email_verified_at"`
}

// Post object for Gorm
type Post struct {
	gorm.Model
	UserID   uint   `gorm:"column:user_id;not null" json:"user_id" validate:"required,min=1"`
	ParentID uint   `gorm:"column:parent_id;not null" json:"parent_id" validate:"required,min=1"`
	Body     string `gorm:"column:body;size:255;not null" json:"body" validate:"required,min=1,max=255"`
	Type     int    `gorm:"column:type;not null;default:1" json:"type" validate:"required,min=1,max=4"`
	Uploads  string `gorm:"column:uploads;size:255;not null" json:"uploads" validate:"required,min=1,max=255"`
	// Post Meta Data Columns
	Tag1ID    uint `gorm:"column:tag1_id;defualt:null" json:"tag1_id" validate:"omitempty,min=1"`
	Tag2ID    uint `gorm:"column:tag2_id;defualt:null" json:"tag2_id" validate:"omitempty,min=1"`
	Tag3ID    uint `gorm:"column:tag3_id;defualt:null" json:"tag3_id" validate:"omitempty,min=1"`
	Liked     int  `gorm:"column:liked;not null;default:0" json:"liked" validate:"omitempty,min=1,max=1"`
	Commented int  `gorm:"column:commented;not null;default:0" json:"commented" validate:"required,min=1,max=1"`
	Viewed    int  `gorm:"column:viewed;not null;default:0" json:"viewed" validate:"required,min=1,max=1"`
}

type Like struct {
	gorm.Model
	UserID uint `gorm:"column:user_id;not null" json:"user_id" validate:"required,min=1"`
	PostID uint `gorm:"column:post_id;not null" json:"post_id" validate:"required,min=1"`
}

type Tag struct {
	gorm.Model
	Name string `gorm:"column:name;size:16;not null" json:"name" validate:"required,min=1,max=16"`
	Slug string `gorm:"column:slug;size:16;not null" json:"slug" validate:"required,min=1,max=16"`
}

// init database migrations if not exist
func InitDbMigrations() {
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Post{})
	db.AutoMigrate(&Like{})
	db.AutoMigrate(&Tag{})
}

/**
*	APP VERSION
 */
// app start time
var startTime = time.Now()
var appVersion = "1.0.0" // -> this will auto update when load from .env

func main() {
	// current directory
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	// load .env file from path.join (process.cwd() + .env)
	err = godotenv.Load(dir + "/.env")
	if err != nil {
		// not found .env file. Log print not fatal
		log.Print("Error loading .env file ENV variables using if exist instead. ", err)
	}

	// get db connection string
	dbConnectionString := os.Getenv("DB_CONN_STRING")
	if dbConnectionString == "" {
		log.Fatal("DB_CONN_STRING is not defined in .env file")
	}

	// init database connection and pool settings
	InitDbConnection(dbConnectionString)
	dbConn, err := db.DB()
	if err != nil {
		log.Println("Error initial connection to database")
		log.Fatal(err)
	}
	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(time.Minute * 5)

	// init database migrations
	InitDbMigrations()

	// init nats connection
	nc, err = InitNatsConnection()
	if err != nil {
		log.Println("Error initial connection to NATS")
		log.Fatal(err)
	}

	// create new gin app
	r := gin.Default()
	// gin maybe behind proxy so we need trust only known proxy
	r.SetTrustedProxies([]string{"0.0.0.0"})

	/**
	*	Security Middleware (Docs: https://github.com/gin-contrib/secure)
	 */
	// allowedHosts : os.Getenv("ALLOWED_HOSTS") then split by comma
	allowedHosts := []string{}
	if os.Getenv("ALLOWED_HOSTS") != "" {
		allowedHosts = strings.Split(os.Getenv("ALLOWED_HOSTS"), ",")
	}
	// sslHost : os.Getenv("SSL_HOST")
	sslHost := os.Getenv("SSL_HOST")
	if sslHost == "" {
		sslHost = "localhost"
	}
	securityConfig := secure.DefaultConfig()
	securityConfig.AllowedHosts = allowedHosts
	securityConfig.SSLHost = sslHost
	// r.Use(secure.New(securityConfig))

	/**
	*	Kernel Status and Memory Info Endpoint
	*	(Docs: https://github.com/appleboy/gin-status-api)
	 */
	// get basic auth credentials from .env file like APP_STAT_AUTH=admin:password
	auth := os.Getenv("APP_STAT_AUTH")
	var statUsername string
	var statPassword string
	if auth != "" {
		authUser := strings.Split(auth, ":")
		statUsername = authUser[0]
		statPassword = authUser[1]
		// if no username or password exit
		if statUsername == "" || statPassword == "" {
			log.Fatal("Error loading APP_STAT_AUTH from .env file")
		}
	}

	/**
	*	ALL APP ENDPOINTS
	 */
	// create memory store for caching (Look to /cache_health)
	store := persistence.NewInMemoryStore(time.Second)

	docs.SwaggerInfo.BasePath = "/v1"
	version := r.Group("/v1")
	{
		/**
		*	--------------- HEALTH ROUTES ---------------
		 */
		status := version.Group("/_")
		{
			// if mode is production disable swagger
			status.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

			status.GET("/app_kernel_stats", AppKernelStatsHandler)

			/**
			 *	Caching Example (Docs: https://github.com/gin-contrib/cache)
			 */
			status.GET("/health", gin.BasicAuth(gin.Accounts{statUsername: statPassword}), AppHealthCheckHandler)
			status.GET("/cache_health", cache.CachePage(store, time.Minute, AppHealthCheckHandler))
		}

		auth_service := version.Group("/auth")
		{
			/**
			*	--------------- APP ROUTES ---------------
			 */
			auth_service.GET("/", GetPostsHandler)
			auth_service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)
		}

		user_service := version.Group("/user")
		{
			/**
			*	--------------- APP ROUTES ---------------
			 */
			user_service.GET("/", GetPostsHandler)
			user_service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)
		}

		post_service := version.Group("/post")
		{
			/**
			*	--------------- APP ROUTES ---------------
			 */
			post_service.GET("/", GetPostsHandler)
			post_service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)
		}

		like_service := version.Group("/like")
		{
			/**
			*	--------------- APP ROUTES ---------------
			 */
			like_service.GET("/", GetPostsHandler)
			like_service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)
		}

		tag_service := version.Group("/tag")
		{
			/**
			*	--------------- APP ROUTES ---------------
			 */
			tag_service.GET("/", GetPostsHandler)
			tag_service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)
		}
	}

	// get app port
	APP_PORT := os.Getenv("APP_PORT")
	if APP_PORT == "" {
		APP_PORT = "8086"
	}
	// start server
	if err := r.Run(":" + APP_PORT); err != nil {
		log.Fatal(err)
	}
}

// AppHealtCheckHandler godoc
// @Summary Returns container kernel info
// @Schemes
// @Description Returns container kernel info
// @Tags app-service-health
// @Security BasicAuth
// @Accept */*
// @Produce json
// @Success 200 {object} object
// @Router /_/app_kernel_stats [get]
func AppKernelStatsHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, osstatus.GetStats())
}

// AppHealtCheckHandler godoc
// @Summary is a simple health check endpoint
// @Schemes
// @Description Checks if app is running and returns container info
// @Tags app-service-health
// @Security BasicAuth
// @Accept */*
// @Produce json
// @Success 200 {object} object
// @Router /_/health [get]
// @Router /_/cache_health [get]
func AppHealthCheckHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":  true,
		"uptime":  time.Since(startTime).String(),
		"version": appVersion,
	})
}

/**
*	--------------- HTTP POST /post Section ---------------
*	1 - Bind Request to DTO
*	2 - Validate DTO
*	3 - Connect to Database
*	4 - Do your database operations
*	5 - Emit event for notify other services for changes
*	6 - Return response
 */
type CreatePostDto struct {
	Body string `json:"body" validate:"required,min=1,max=255"`
}

/**
*	CreatePostDtoValidator : Validate CreatePostDto
*	Returns createPostDto,error
 */
func CreatePostDtoValidator(ctx *gin.Context) (CreatePostDto, error) {
	/*
		// check user permission
		userRole := "user" // TODO: get user role from context and jwt
		canWatch, _ := userRole.Can("post", "create")
		fmt.Printf("Can watch %s? %t\n", rating, canWatch)
	*/

	var createPostDto CreatePostDto
	// cast to json
	if err := ctx.BindJSON(&createPostDto); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"type":    "create-post/request-body",
			"message": err.Error(),
		})
		// return error
		return createPostDto, err
	}
	// validate
	validateDto := validator.New()
	if err := validateDto.Struct(createPostDto); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"type":    "create-post/validation",
			"message": err.Error(),
		})
		return createPostDto, err
	}
	// return createPostDto
	return createPostDto, nil
}

// CreatePostHandler godoc
// @Summary Create Post by CreatePostDto
// @Schemes
// @Description Create Post by CreatePostDto
// @Tags post-service
// @Security BearerAuth
// @Body CreatePostDto
// @Accept application/json
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 422 {object} object
// @Router /post/ [post]
func CreatePostHandler(ctx *gin.Context) {
	// validate request
	createPostDto, err := CreatePostDtoValidator(ctx)
	if err != nil {
		return
	}

	// create new product
	post := Post{
		Body: createPostDto.Body,
	}

	// save to database
	db.Create(&post)
	if post.ID == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{
			"status":  false,
			"type":    "create-post/save",
			"message": "Unprocessable inputs ensured.",
		})
		return
	}

	// fire event for notify other services for changes
	// Simple Publisher
	nc.Publish("post.created", []byte("Post Created Body: "+post.Body))

	// return post
	ctx.JSON(http.StatusOK, gin.H{
		"post": post,
	})
}

/**
*	--------------- HTTP Get /post Section ---------------
*	1 - Get Pagination values
*	3 - Connect to Database
*	4 - Do your database operations
*	5 - Emit event for notify other services for changes
*	6 - Return response
 */

// GetPostsHandler godoc
// @Summary Get Posts
// @Schemes
// @Description Get Posts with limit and page
// @Tags post-service
// @Param limit query int false "limit"
// @Param page query int false "page"
// @Accept application/json
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 422 {object} object
// @Failure 500 {object} object
// @Router /post/ [get]
func GetPostsHandler(ctx *gin.Context) {
	// get pagination params page should be 1<=page<100 and limit should be 1<=limit<50
	limitQ := ctx.DefaultQuery("limit", "10")
	if limitQ == "" || limitQ < "1" || limitQ > "100" {
		limitQ = "10"
	}
	pageQ := ctx.DefaultQuery("page", "1")
	if pageQ == "" || pageQ < "1" || pageQ > "100" {
		pageQ = "1"
	}

	// cast to int
	limit, _ := strconv.Atoi(limitQ)
	page, _ := strconv.Atoi(pageQ)
	offset := (page - 1) * limit

	// get all posts by limit and offset
	var posts []Post
	db.Limit(limit).Offset(offset).Find(&posts)

	// fire event for notify other services for changes
	nc.Publish("post.select", []byte("Post Got by ip: "+ctx.ClientIP()))

	// return posts
	ctx.JSON(http.StatusOK, gin.H{
		"posts": posts,
	})
}
