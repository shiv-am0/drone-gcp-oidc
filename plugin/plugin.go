// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	OIDCToken  string `envconfig:"PLUGIN_OIDC_TOKEN_ID"`
	ProjectID  string `envconfig:"PLUGIN_PROJECT_ID"`
	PoolID     string `envconfig:"PLUGIN_POOL_ID"`
	ProviderID string `envconfig:"PLUGIN_PROVIDER_ID"`
	ServiceAcc string `envconfig:"PLUGIN_SERVICE_ACCOUNT_EMAIL_ID"`
	Duration   string `envconfig:"PLUGIN_DURATION"`
}

// Exec executes the plugin.
func Exec(ctx context.Context, args Args) error {
	if err := VerifyEnv(args); err != nil {
		return err
	}

	if args.Duration == "" {
		args.Duration = "3600s"
	} else {
		args.Duration = args.Duration + "s"
	}

	federalToken, err := GetFederalToken(args.OIDCToken, args.ProjectID, args.PoolID, args.ProviderID)
	if err != nil {
		return err
	}

	accessToken, err := GetGoogleCloudAccessToken(federalToken, args.ServiceAcc, args.Duration)

	if err != nil {
		return err
	}

	logrus.Infof("acess token retrieved successfully\n")
	logrus.Infof("access token set as GCLOUD_ACCESS_TOKEN\n")

	if err := WriteEnvToFile(accessToken); err != nil {
		return err
	}

	logrus.Infof("access token written to env\n")

	return nil
}

func VerifyEnv(args Args) error {
	if args.OIDCToken == "" {
		return fmt.Errorf("oidc-token is not provided")
	}
	if args.ProjectID == "" {
		return fmt.Errorf("project-id is not provided")
	}
	if args.PoolID == "" {
		return fmt.Errorf("pool-id is not provided")
	}
	if args.ProviderID == "" {
		return fmt.Errorf("provider-id is not provided")
	}
	if args.ServiceAcc == "" {
		return fmt.Errorf("service account email is not provided")
	}
	return nil
}

func WriteEnvToFile(value string) error {
	outputFile, err := os.OpenFile(os.Getenv("DRONE_OUTPUT"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}

	defer outputFile.Close()

	_, err = fmt.Fprintf(outputFile, "GCLOUD_ACCESS_TOKEN=%s\n", value)
	if err != nil {
		return fmt.Errorf("failed to write to env: %w", err)
	}

	return nil
}
