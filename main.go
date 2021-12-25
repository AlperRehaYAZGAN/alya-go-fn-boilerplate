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

// @host api.pethackathon.yazgan.xyz
// @BasePath /v1


// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.basic BasicAuth
// @in header
// @name Authentication


// @title Development Branch Transaction Service
// @version 1.0
// @description This is a sample server for Transaction Service.

// @contact.name Development Branch
// @contact.url https://git.yazgan.xyz/alperreha/
// @contact.email support@alperreha.yazgan.xyz

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT


import (
	// system packages
    "net/http"
	"time"
	"log"
	"strconv"
	"os"
	"strings"
	// json
	"encoding/json"

	// third party packages
	"github.com/joho/godotenv"
	osstatus "github.com/fukata/golang-stats-api-handler"
	"git.yazgan.xyz/anadolusigorta-pethackathon/service-location-notification/docs"
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
					rbac.NewGlobPermission("transaction", "*"),
			},
	},
	{
			RoleID: "User",
			Permissions: []rbac.Permission{
				rbac.NewGlobPermission("transaction", "read"),
				rbac.NewGlobPermission("transaction", "create"),
				rbac.NewGlobPermission("transaction", "delete"),
			},
	},
	{
			RoleID: "Guest",
			Permissions: []rbac.Permission{
				rbac.NewGlobPermission("transaction", "read"),
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

func InitDbConnection(dbConnString string) {
    var err error
	//sqlite
    db, err = gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	// postgres
	//db, err = gorm.Open(postgres.Open(dbConnString), &gorm.Config{})
    if err != nil {
        log.Panic(err)
    }
}


// Transaction object for Gorm
type Transaction struct {
	gorm.Model
	OwnerID int `gorm:"column:owner_id;not null" json:"owner_id" validate:"required,min=1"`
	PetID int `gorm:"column:pet_id;not null" json:"pet_id" validate:"required,min=1"`
	Status int `gorm:"column:status;type:integer;default:1" json:"status" validate:"required,min=1"`
	Message string `gorm:"column:message;size:255;not null" json:"message" validate:"required,min=1,max=255"`
	Latitude float64 `gorm:"column:latitude;type:float;default:null" json:"latitude" validate:"required"`
	Longitude float64 `gorm:"column:longitude;type:float;default:null" json:"longitude" validate:"required"`
	Upload string `gorm:"column:upload;size:255;not null" json:"upload" validate:"required,min=1,max=255"`
}


// init database migrations if not exist
func InitDbMigrations() {
	db.AutoMigrate(&Transaction{})
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
		log.Print("Error loading .env file ENV variables using if exist instead. ",err)
	}

	// get db connection string
	dbConnectionString := os.Getenv("DB_CONN_STRING")
	if dbConnectionString == "" {
		log.Fatal("DB_CONN_STRING is not defined in .env file")
	}

	// get app version from .env file
	appVersion = os.Getenv("APP_VERSION")
	if appVersion == "" {
		appVersion = "1.0.0"
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
		service := version.Group("/transaction")
		{
			/**
			*	--------------- APP ROUTES ---------------
			*/
			service.GET("/", GetTransactionsHandler)
			service.POST("/", CreateTransactionHandler)
			service.GET("/:id", GetTxByIdHandler)

			/**
			*	--------------- HEALTH ROUTES ---------------
			*/
			status := service.Group("/_") 
			{
				// if mode is production disable swagger
				status.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

				status.GET("/app_kernel_stats", AppKernelStatsHandler)

				/**
				*	Caching Example (Docs: https://github.com/gin-contrib/cache)
				*/
				status.GET("/health", gin.BasicAuth(gin.Accounts{ statUsername : statPassword }) ,AppHealthCheckHandler)
				status.GET("/cache_health", cache.CachePage(store, time.Minute,AppHealthCheckHandler))
			}
		}			
	}



	// get app port
	APP_PORT := os.Getenv("APP_PORT")
	if APP_PORT == "" {
		APP_PORT = "9092"
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
// @Tags transaction-service-health
// @Security BasicAuth
// @Accept */*
// @Produce json
// @Success 200 {object} object
// @Router /transaction/_/app_kernel_stats [get]
func AppKernelStatsHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, osstatus.GetStats())
}


// AppHealtCheckHandler godoc
// @Summary is a simple health check endpoint
// @Schemes 
// @Description Checks if app is running and returns container info
// @Tags transaction-service-health
// @Security BasicAuth
// @Accept */*
// @Produce json
// @Success 200 {object} object
// @Router /transaction/_/health [get]
// @Router /transaction/_/cache_health [get]
func AppHealthCheckHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": true,
		"uptime": time.Since(startTime).String(),
		"version": appVersion,
	})
}

/**
*	--------------- HTTP POST /transaction Section ---------------
*	1 - Bind Request to DTO
*	2 - Validate DTO
*	3 - Connect to Database
*	4 - Do your database operations
*	5 - Emit event for notify other services for changes
*	6 - Return response
*/
type CreateTransactionDto struct {
	OwnerID int /**/ `json:"owner_id" validate:"required,min=1"` /**/
	PetID int `json:"pet_id" validate:"required,min=1"`
	Status int `json:"status" validate:"required,min=1,max=10"`
	Message string `json:"message" validate:"required,min=1,max=255"`
	Latitude float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" validate:"required,min=-180,max=180"`
	Upload string `json:"upload" validate:"required,min=1,max=255"`
}


/**
*	CreateTransactionDtoValidator : Validate CreateTransactionDto
*	Returns createTxDto,error
*/
func CreateTransactionDtoValidator(ctx *gin.Context) (CreateTransactionDto,error) {
	var createTxDto CreateTransactionDto
	// cast to json
    if err := ctx.BindJSON(&createTxDto); err != nil {
        ctx.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"type": "create-tx/request-body",
            "message": err.Error(),
        })
		// return error
		return createTxDto,err
    }
	// validate
	validateDto := validator.New()
	if err := validateDto.Struct(createTxDto); err != nil {
        ctx.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"type": "create-tx/validation",
			"message": err.Error(),
        })
		return createTxDto,err
    }
	// return createTxDto
	return createTxDto,nil
}



// CreateTransactionHandler godoc
// @Summary Create Transaction by CreateTransactionDto
// @Schemes 
// @Description Create Transaction by CreateTransactionDto
// @Tags transaction-service
// @Security BearerAuth
// @Body CreateTransactionDto
// @Accept application/json
// @Produce json
// @Param body body CreateTransactionDto true "Create Transaction Dto"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 422 {object} object
// @Router /transaction/ [post]
func CreateTransactionHandler(ctx *gin.Context) {
	// validate request
	createTxDto,err := CreateTransactionDtoValidator(ctx)
	if err != nil { return }

	/*
	// get bearer token from header
	bearerToken := ctx.GetHeader("Authorization")
	if bearerToken == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"status": false,
			"type": "create-tx/auth",
			"message": "Bearer token not found",
		})
		return
	}

	// get userid from NATS.Request from Auth Service. Service return userid as string or "false" as string
	userID,err := nats.Request("user.isvalid",bearerToken, time.Second*5)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"status": false,
			"type": "create-tx/auth",
			"message": "Bearer token not found",
		})
		return
	}
	// convert userid to int
	userIDInt,err := strconv.Atoi(string(userID.Data))
	if ((err != nil) || (userIDInt == 0)) {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"status": false,
			"type": "create-tx/auth",
			"message": "Bearer token not found",
		})
		return
	}
	*/
	
	// create new transaction
	tx := Transaction{
		OwnerID: /* userIDInt */ createTxDto.OwnerID,
		PetID: createTxDto.PetID,
		Status: createTxDto.Status,
		Message: createTxDto.Message,
		Latitude: createTxDto.Latitude,
		Longitude: createTxDto.Longitude,
		Upload: createTxDto.Upload,
	}

	// save to database
	db.Create(&tx)
	if tx.ID == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": false,
			"type": "create-tx/save",
			"message": "Unprocessable inputs ensured.",
		})
		return
	}

	// fire event for notify other services for changes
	// Simple Publisher
	nc.Publish("tx.created", []byte("Transaction Created Message: " + tx.Message))

	// return transaction
	ctx.JSON(http.StatusOK, gin.H{
		"status": true,
		"type": "create-tx/success",
		"message": "Transaction Created Successfully",
		"transaction": tx,
	})
}



/**
*	--------------- HTTP Get /transaction Section ---------------
*	1 - Get Pagination values
*	3 - Connect to Database
*	4 - Do your database operations
*	5 - Emit event for notify other services for changes
*	6 - Return response
*/




// GetTransactionsHandler godoc
// @Summary Get Transactions
// @Schemes 
// @Description Get Transactions with limit and page
// @Tags transaction-service
// @Accept application/json
// @Produce json
// @Param limit query int false "limit"
// @Param page query int false "page"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 422 {object} object
// @Failure 500 {object} object
// @Router /transaction/ [get]
func GetTransactionsHandler(ctx *gin.Context) {
	// get pagination params page should be 1<=page<100 and limit should be 1<=limit<50
	limitQ := ctx.DefaultQuery("limit", "10")
	if(limitQ == "" || limitQ < "1" || limitQ > "100") { limitQ = "10" } 
	pageQ := ctx.DefaultQuery("page", "1")
	if(pageQ == "" || pageQ < "1" || pageQ > "100") { pageQ = "1" }

	// cast to int
	limit,_ := strconv.Atoi(limitQ)
	page,_ := strconv.Atoi(pageQ)
	if(page < 1) { page = 1 }
	offset := (page - 1) * limit

	// get all txs by limit and offset
	var txs []Transaction
	db.Limit(limit).Offset(offset).Find(&txs)

	// fire event for notify other services for changes
	nc.Publish("tx.select", []byte("Tx Got by ip: " + ctx.ClientIP()))

	// return transactions
	ctx.JSON(http.StatusOK, gin.H{
		"transactions": txs,
	})
}



// GetTxByIdHandler godoc
// @Summary Get Transaction by ID
// @Schemes 
// @Description Get Transaction by ID
// @Tags transaction-service
// @Accept application/json
// @Produce json
// @Param id path string true "Transaction ID"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 500 {object} object
// @Router /transaction/{id} [get]
func GetTxByIdHandler(ctx *gin.Context) {
	// get Transaction id from url params like /transaction/:id 
	txIdQ := ctx.Param("id")
	// cast to int
	txId,err := strconv.Atoi(txIdQ)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"type": "get-tx/validation",
			"message": "Invalid Transaction ID",
		})
		return
	}

	// get transaction by id
	var tx Transaction
	db.First(&tx, txId)
	if tx.ID == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{
			"status": false,
			"type": "get-tx/not-found",
			"message": "Transaction not found",
		})
		return
	}

	// fire event for notify other services for changes
	nc.Publish("tx.select", []byte("Transaction got by ip: " + ctx.ClientIP()))


	// get user.info
	userIdStr := strconv.Itoa(tx.OwnerID)
	msgUser, errU := nc.Request("user.info", []byte(userIdStr) , time.Second)
	// get pet.info
	petIdStr := strconv.Itoa(tx.PetID)
	msgPet, errP := nc.Request("pet.info", []byte(petIdStr) , time.Second)

	// set any json data from user.info and pet.info
	var userInfo map[string]interface{}
	var petInfo map[string]interface{}
	
	// check errors
	if errU != nil {
		userInfo = nil
	} else {
		// decode msgUser.Data to json
		err = json.Unmarshal(msgUser.Data, &userInfo)
		if err != nil {
			userInfo = nil
		} 
	}
	// check errors
	if errP != nil {
		petInfo = nil
	} else {
		// decode msgPet.Data to json
		err = json.Unmarshal(msgPet.Data, &petInfo)
		if err != nil {
			petInfo = nil
		} 
	}

	// return transaction
	ctx.JSON(http.StatusOK, gin.H{
		"transaction": tx,
		"user": userInfo,
		"pet": petInfo,
	})
}
