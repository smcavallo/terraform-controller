package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ibuildthecloud/terraform-operator/pkg/executor/runner"
	"github.com/ibuildthecloud/terraform-operator/pkg/git"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if os.Getenv("EXECUTOR_DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	err := run()

	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var config *rest.Config
	var err error

	// Useful for running executor locally without having to deploy to k8s
	if path := os.Getenv("KUBECONFIG"); path != "" {
		logrus.Info(path)

		config, err = clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			return err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return err
		}
	}

	runner, err := runner.NewRunner(config)
	if err != nil {
		return err
	}

	err = runner.Populate()
	if err != nil {
		return err
	}

	err = git.CloneRepo(context.Background(), runner.ExecutionRun.Spec.Content.Git.URL, runner.ExecutionRun.Spec.Content.Git.Commit, runner.GitAuth)
	if err != nil {
		return err
	}

	err = runner.WriteConfigFile()
	if err != nil {
		return err
	}

	err = runner.WriteVarFile()
	if err != nil {
		return err
	}

	out, err := runner.TerraformInit()
	if err != nil {
		return err
	}

	logrus.Info(out)

	switch runner.Action {
	case "create":
		out, err = runner.Create()
		if err != nil {
			return err
		}
		logrus.Info(out)

		runner.SetExecutionRunStatus("applied")

		err = runner.SaveOutputs()
		if err != nil {
			return err
		}
	case "destroy":
		out, err = runner.Destroy()
		if err != nil {
			return err
		}
		logrus.Info(out)
	default:
		return errors.New("action is not valid, ony 'create' or 'destroy' allowed")
	}

	//TODO: delete this
	fmt.Println("Sleeping for 10 min so you can look at me....")
	time.Sleep(10 * time.Minute)

	return nil
}