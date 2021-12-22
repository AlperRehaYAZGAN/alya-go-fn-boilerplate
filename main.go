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
// @BasePath /v1


import (
	// system packages
    "net/http"
	"time"
	"log"
	"strconv"
	"os"
	"strings"

	// third party packages
	"github.com/joho/godotenv"
	osstatus "github.com/fukata/golang-stats-api-handler"
	docs "git.yazgan.xyz/alperreha/alya-go-fn-boilerplate/docs"
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
	"github.com/zpatrick/rbac"
	// validator packages
	"github.com/go-playground/validator/v10"
	// database packages
	"gorm.io/gorm"
	// "gorm.io/driver/postgres" 
	"gorm.io/driver/sqlite"
	// event packages
	// go get github.com/nats-io/nats.go/@v1.13.0
	"github.com/nats-io/nats.go"

)


/**
*	App RBAC Definitions
*/
var APP_ROLES = []rbac.Role{
	{
			RoleID: "Admin",
			Permissions: []rbac.Permission{
					rbac.NewGlobPermission("post", "*"),
			},
	},
	{
			RoleID: "User",
			Permissions: []rbac.Permission{
				rbac.NewGlobPermission("post", "read"),
				rbac.NewGlobPermission("post", "create"),
				rbac.NewGlobPermission("post", "delete"),
			},
	},
	{
			RoleID: "Guest",
			Permissions: []rbac.Permission{
				rbac.NewGlobPermission("post", "read"),
			},
	},
}

/*
for _, role := range roles {
	fmt.Println("Role:", role.RoleID)
	for _, rating := range []string{"g", "pg-13", "r"} {
			canWatch, _ := role.Can("watch", rating)
			fmt.Printf("Can watch %s? %t\n", rating, canWatch)
	}
}
*/

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

func InitDbConnection() {
    var err error
    db, err = gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
    if err != nil {
        log.Panic(err)
    }
}


// Post object for Gorm
type Post struct {
	gorm.Model
	Body string `gorm:"column:body;size:255;not null" json:"body" validate:"required,min=1,max=255"`
}


// init database migrations if not exist
func InitDbMigrations() {
	db.AutoMigrate(&Post{})
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
		log.Fatal("Error loading .env file")
	}

	// init database connection and pool settings
	InitDbConnection()
	dbConn, err := db.DB()
	if err != nil {
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
		log.Fatal(err)
	}


	/**
	*	Connect to Nats and Register Event Listener
	*/
	/*		
	-----------------------------------------------------
	THIS IS NOT NEEDED FOR THIS APP BUT BOILERPLATE SHOULD STAY
	-----------------------------------------------------
	// Simple Async Subscriber
	nc.Subscribe("post.created", func(m *nats.Msg) {
		log.Println("Received a post.created:", string(m.Data))
	})

	nc.Subscribe("post.select", func(m *nats.Msg) {
		log.Println("Received a post.select:", string(m.Data))
	})
	*/

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
		allowedHosts = strings.Split(os.Getenv("ALLOWED_HOSTS"),",")
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
		authUser := strings.Split(auth,":")
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
		service := version.Group("/post")
		{
			/**
			*	--------------- APP ROUTES ---------------
			*/
			service.GET("/", GetPostHandler)
			service.POST("/", CreatePostHandler)
			//service.GET("/:id", GetPostByIdHandler)

			/**
			*	--------------- HEALTH ROUTES ---------------
			*/
			status := service.Group("/_") 
			{
				// if mode is production disable swagger
				if os.Getenv("APP_ENV") != "production" {
					status.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
				}

				status.GET("/app_kernel_stats", gin.BasicAuth(gin.Accounts{ statUsername : statPassword }) ,func (ctx *gin.Context) {
					ctx.JSON(http.StatusOK, osstatus.GetStats())
				})

				/**
				*	Caching Example (Docs: https://github.com/gin-contrib/cache)
				*/
				status.GET("/health", AppHealthCheckHandler)
				status.GET("/cache_health", cache.CachePage(store, time.Minute,AppHealthCheckHandler))
			}
		}

			
	}


	// get app port
	APP_PORT := os.Getenv("APP_PORT")
	if APP_PORT == "" {
		APP_PORT = "9090"
	}
	// start server
	if err := r.Run(":" + APP_PORT); err != nil {
		log.Fatal(err)
	}
}


// AppHealtCheckHandler is a simple health check endpoint
// @Summary App Healt Check
// @Schemes 
// @Description Checks if app is running and returns container info
// @Tags post-service
// @Accept */*
// @Produce json
// @Success 200 {object} object
// @Router /v1/post/_/health [get]
// @Router /v1/post/_/cache_health [get]
func AppHealthCheckHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": true,
		"uptime": time.Since(startTime).String(),
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
func CreatePostDtoValidator(ctx *gin.Context) (CreatePostDto,error) {
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
			"status": false,
			"type": "create-post/request-body",
            "message": err.Error(),
        })
		// return error
		return createPostDto,err
    }
	// validate
	validateDto := validator.New()
	if err := validateDto.Struct(createPostDto); err != nil {
        ctx.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"type": "create-post/validation",
			"message": err.Error(),
        })
		return createPostDto,err
    }
	// return createPostDto
	return createPostDto,nil
}



// CreatePostHandler : Create Post
// @Summary Creates post by given CreatePostDto
// @Schemes 
// @Description Checks if app is running and returns container info
// @Tags post-service
// @Accept application/json
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 422 {object} object
// @Router /v1/post/ [post]
func CreatePostHandler(ctx *gin.Context) {
	// validate request
	createPostDto,err := CreatePostDtoValidator(ctx)
	if err != nil { return }		
	
	// create new product
	post := Post{
		Body: createPostDto.Body,
	}

	// save to database
	db.Create(&post)
	if post.ID == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": false,
			"type": "create-post/save",
			"message": "Unprocessable inputs ensured.",
		})
		return
	}

	// fire event for notify other services for changes
	// Simple Publisher
	nc.Publish("post.created", []byte("Post Created Body: " + post.Body))

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
func GetPostHandler(ctx *gin.Context) {
	// get pagination params page should be 1<=page<100 and limit should be 1<=limit<50
	limitQ := ctx.DefaultQuery("limit", "10")
	if(limitQ == "" || limitQ < "1" || limitQ > "100") { limitQ = "10" } 
	pageQ := ctx.DefaultQuery("page", "1")
	if(pageQ == "" || pageQ < "1" || pageQ > "100") { pageQ = "1" }

	// cast to int
	limit,_ := strconv.Atoi(limitQ)
	page,_ := strconv.Atoi(pageQ)
	offset := (page - 1) * limit

	// get all posts by limit and offset
	var posts []Post
	db.Limit(limit).Offset(offset).Find(&posts)

	// fire event for notify other services for changes
	nc.Publish("post.select", []byte("Post Got by ip: " + ctx.ClientIP()))

	// return posts
	ctx.JSON(http.StatusOK, gin.H{
		"posts": posts,
	})
}
