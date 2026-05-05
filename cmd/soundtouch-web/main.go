// Package main provides a web UI for controlling Bose SoundTouch devices.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gesellix/bose-soundtouch/pkg/service/soundtouchweb"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "soundtouch-web",
		Usage: "Web UI for controlling Bose SoundTouch devices",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "HTTP port to listen on",
				Value:   "8080",
				EnvVars: []string{"PORT"},
			},
			&cli.StringFlag{
				Name:    "bind",
				Usage:   "Network interface to bind to",
				EnvVars: []string{"BIND_ADDR"},
			},
		},
		Action: func(c *cli.Context) error {
			port := c.String("port")
			bindAddr := c.String("bind")

			addr := ":" + port
			if bindAddr != "" {
				addr = bindAddr + ":" + port
			}

			webApp := soundtouchweb.New()

			r := chi.NewRouter()
			webApp.Mount(r)

			log.Printf("SoundTouch Web UI starting on http://%s", addr)

			return http.ListenAndServe(addr, r)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
