package main

import (
	"context"
	"os"

	"github.com/PastureStack/storage-plugins/runtime/nfs/docker/volumeplugin"
	dockerClient "github.com/docker/engine-api/client"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/kubernetes-agent/healthcheck"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "pasturestack-nfs"
	app.Version = VERSION
	app.Usage = "Provide NFS volumes to the compatible control plane"
	app.Action = func(c *cli.Context) error {
		return start(c)
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "driver-name",
			Usage: "The volume driver name",
		},
		cli.StringFlag{
			Name:   "cattle-url",
			Usage:  "Compatible control-plane API URL",
			EnvVar: "CATTLE_URL",
		},
		cli.StringFlag{
			Name:   "cattle-access-key",
			Usage:  "The access key required to authenticate with cattle server",
			EnvVar: "CATTLE_ACCESS_KEY",
		},
		cli.StringFlag{
			Name:   "cattle-secret-key",
			Usage:  "The secret key required to authenticate with cattle server",
			EnvVar: "CATTLE_SECRET_KEY",
		},
		cli.IntFlag{
			Name:  "healthcheck-interval",
			Value: 5000,
			Usage: "set the frequency of performing healthchecks",
		},
		cli.IntFlag{
			Name:  "healthcheck-port",
			Usage: "listen port for healthchecks",
		},
		cli.StringFlag{
			Name:   "docker-host",
			Value:  "unix:///var/run/docker.sock",
			Usage:  "The DOCKER_HOST to connect to",
			EnvVar: "DOCKER_HOST",
		},
		cli.StringFlag{
			Name:   "docker-api-version",
			Value:  "",
			Usage:  "The Docker API version used by the legacy engine-api client; empty sends unversioned requests",
			EnvVar: "DOCKER_API_VERSION",
		},
		cli.BoolFlag{
			Name:  "save-on-attach",
			Usage: "Save the volume to the compatible control plane on attach",
		},
	}
	logrus.Info("Starting PastureStack NFS storage driver")
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func start(c *cli.Context) error {
	logrus.Info("Starting")
	cli, err := dockerClient.NewClient(c.String("docker-host"), c.String("docker-api-version"), nil, nil)
	if err != nil {
		return err
	}

	if _, err := cli.Info(context.Background()); err != nil {
		return err
	}

	opts := &client.ClientOpts{
		Url:       c.String("cattle-url"),
		AccessKey: c.String("cattle-access-key"),
		SecretKey: c.String("cattle-secret-key"),
	}
	client, err := client.NewRancherClient(opts)
	if err != nil {
		return err
	}

	driverName := c.String("driver-name")
	if driverName == "" {
		return errors.New("--driver-name is required")
	}
	d, err := volumeplugin.NewControlPlaneStorageDriver(driverName, client, cli)
	if err != nil {
		return err
	}

	d.SaveOnAttach = c.Bool("save-on-attach")

	logrus.Infof("Starting plugin for %s", driverName)
	h := volume.NewHandler(d)
	if c.Int("healthcheck-port") > 0 {
		go func() {
			err := healthcheck.StartHealthCheck(c.Int("healthcheck-port"))
			logrus.Fatalf("Error while running healthcheck [%v]", err)
		}()
	}
	volumeplugin.ExtendHandler(h, d)
	if err := volumeplugin.ForceSymlinkInDockerPlugins(driverName); err != nil {
		return errors.Wrap(err, "registering Docker volume plugin")
	}
	return h.ServeUnix("root", volumeplugin.ControlPlaneSocketFile(driverName))
}
