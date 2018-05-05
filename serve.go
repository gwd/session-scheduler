package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// URL scheme
// /register
// /login
// /uid/{disc,usr}/$uid/{view,edit}
// /new/discussion
// /new/user
// /list/discussions
// /list/users
// /admin/{console,test}
// 

func serve() {
	router := NewRouter()

	router.GET("/", HandleHome)
	router.GET("/register", HandleUserNew)
	router.POST("/register", HandleUserCreate)
	router.GET("/login", HandleSessionNew)
	router.POST("/login", HandleSessionCreate)

	router.GET("/discussion/notfound", HandleDiscussionNotFound)

	router.GET("/schedule", HandleScheduleView)

	router.GET("/list/:itype", HandleList)
	router.GET("/uid/:itype/:uid/:action", HandleUid)
	router.POST("/uid/:itype/:uid/:action", HandleUidPost)

	router.ServeFiles(
		"/assets/*filepath",
		http.Dir("assets/"),
	)

	secureRouter := NewRouter()
	secureRouter.GET("/sign-out", HandleSessionDestroy)
	secureRouter.GET("/account", HandleUserEdit)
	secureRouter.POST("/account", HandleUserUpdate)
	secureRouter.GET("/discussion/new", HandleDiscussionNew)
	secureRouter.POST("/discussion/new", HandleDiscussionCreate)
	secureRouter.GET("/admin/:template", HandleAdminConsole)
	secureRouter.POST("/admin/runschedule", HandleAdminRunSchedule)

	secureRouter.POST("/testaction/:action", HandleTestAction)

	
	middleware := Middleware{}
	middleware.Add(router)
	middleware.Add(http.HandlerFunc(RequireLogin))
	middleware.Add(secureRouter)

	log.Fatal(http.ListenAndServe("localhost:3000", middleware))
}

// Creates a new router
func NewRouter() *httprouter.Router {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	return router
}
