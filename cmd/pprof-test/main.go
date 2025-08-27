package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	go func() {
		log.Println("Starting pprof server on localhost:6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	srv := echo.New()
	srv.Use(middleware.Logger())
	srv.Use(middleware.Recover())
	srv.HideBanner = true
	srv.HidePort = true
	srv.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	log.Println("Starting web application server on localhost:8080")
	srv.Logger.Fatal(srv.Start(":8080"))
}
