package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PastureStack/storage-plugins/runtime/nfs/docker/volumeplugin"
	"github.com/PastureStack/storage-plugins/runtime/nfs/internal/controlplane"
	"github.com/docker/go-plugins-helpers/volume"
	dockerClient "github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := &cli.Command{
		Name:    "pasturestack-storage-runtime",
		Version: VERSION,
		Usage:   "Provide managed storage volumes to the compatible control plane",
		Action:  start,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "driver-name",
				Usage: "The volume driver name",
			},
			&cli.StringFlag{
				Name:    "cattle-url",
				Usage:   "Compatible control-plane API URL",
				Sources: cli.EnvVars("CATTLE_URL"),
			},
			&cli.StringFlag{
				Name:    "cattle-access-key",
				Usage:   "The access key required to authenticate with cattle server",
				Sources: cli.EnvVars("CATTLE_ACCESS_KEY"),
			},
			&cli.StringFlag{
				Name:    "cattle-secret-key",
				Usage:   "The secret key required to authenticate with cattle server",
				Sources: cli.EnvVars("CATTLE_SECRET_KEY"),
			},
			&cli.IntFlag{
				Name:  "healthcheck-interval",
				Value: 5000,
				Usage: "set the frequency of performing healthchecks",
			},
			&cli.IntFlag{
				Name:  "healthcheck-port",
				Usage: "listen port for healthchecks",
			},
			&cli.StringFlag{
				Name:    "docker-host",
				Value:   "unix:///var/run/docker.sock",
				Usage:   "The DOCKER_HOST to connect to",
				Sources: cli.EnvVars("DOCKER_HOST"),
			},
			&cli.StringFlag{
				Name:    "docker-api-version",
				Value:   "",
				Usage:   "The Docker API version; empty enables automatic negotiation",
				Sources: cli.EnvVars("DOCKER_API_VERSION"),
			},
			&cli.BoolFlag{
				Name:  "save-on-attach",
				Usage: "Save the volume to the compatible control plane on attach",
			},
		},
	}
	logrus.Info("Starting PastureStack storage driver")
	if err := app.Run(context.Background(), os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func start(ctx context.Context, c *cli.Command) error {
	logrus.Info("Starting")
	dockerOpts := []dockerClient.Opt{dockerClient.WithHost(c.String("docker-host"))}
	if version := c.String("docker-api-version"); version != "" {
		dockerOpts = append(dockerOpts, dockerClient.WithAPIVersion(version))
	} else {
		dockerOpts = append(dockerOpts, dockerClient.WithAPIVersionNegotiation())
	}
	docker, err := dockerClient.New(dockerOpts...)
	if err != nil {
		return err
	}
	defer docker.Close()

	if _, err := docker.Info(ctx, dockerClient.InfoOptions{}); err != nil {
		return err
	}

	opts := &controlplane.ClientOpts{
		URL:       c.String("cattle-url"),
		AccessKey: c.String("cattle-access-key"),
		SecretKey: c.String("cattle-secret-key"),
	}
	controlPlane, err := controlplane.NewClient(opts)
	if err != nil {
		return err
	}

	driverName := c.String("driver-name")
	if driverName == "" {
		return fmt.Errorf("--driver-name is required")
	}
	d, err := volumeplugin.NewControlPlaneStorageDriver(driverName, controlPlane, docker)
	if err != nil {
		return err
	}

	d.SaveOnAttach = c.Bool("save-on-attach")

	logrus.Infof("Starting plugin for %s", driverName)
	h := volume.NewHandler(d)
	if c.Int("healthcheck-port") > 0 {
		go func() {
			err := startHealthCheck(c.Int("healthcheck-port"))
			logrus.Fatalf("Error while running healthcheck [%v]", err)
		}()
	}
	volumeplugin.ExtendHandler(h, d)
	if err := volumeplugin.ForceSymlinkInDockerPlugins(driverName); err != nil {
		return fmt.Errorf("registering Docker volume plugin: %w", err)
	}
	return h.ServeUnix(volumeplugin.ControlPlaneSocketFile(driverName), 0)
}
