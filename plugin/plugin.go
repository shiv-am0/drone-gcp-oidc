// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	OIDCToken   string `envconfig:"PLUGIN_OIDC_TOKEN_ID"`
	ProjectID   string `envconfig:"PLUGIN_PROJECT_ID"`
	PoolID      string `envconfig:"PLUGIN_POOL_ID"`
	ProviderID  string `envconfig:"PLUGIN_PROVIDER_ID"`
	ServiceAcc  string `envconfig:"PLUGIN_SERVICE_ACCOUNT_EMAIL_ID"`
	Duration    string `envconfig:"PLUGIN_DURATION"`
	CreateCreds bool   `envconfig:"PLUGIN_CREATE_DEFAULT_CREDENTIALS_FILE"`
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

	if args.CreateCreds {
		credsPath, err := WriteApplicationDefaultCredentials(federalToken, args.Duration, args.ProjectID, args.PoolID, args.ProjectID, args.ServiceAcc)
		if err != nil {
			return err
		}
		logrus.Infof("credentials file written to %s\n", credsPath)
	} else {
		accessToken, err := GetGoogleCloudAccessToken(federalToken, args.ServiceAcc, args.Duration)

		if err != nil {
			return err
		}

		logrus.Infof("acess token retrieved successfully\n")

		// Export access token to CLOUDSDK_AUTH_ACCESS_TOKEN env
		err = os.Setenv("CLOUDSDK_AUTH_ACCESS_TOKEN", accessToken)
		if err != nil {
			return fmt.Errorf("failed to set CLOUDSDK_AUTH_ACCESS_TOKEN env variable: %w", err)
		}

		logrus.Infof("access token set as GCLOUD_ACCESS_TOKEN\n")

		if err := WriteEnvToFile("GCLOUD_ACCESS_TOKEN", accessToken); err != nil {
			return err
		}

		logrus.Infof("access token written to env\n")
	}

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

func WriteEnvToFile(key, value string) error {
	outputFile, err := os.OpenFile(os.Getenv("DRONE_OUTPUT"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}

	defer outputFile.Close()

	_, err = fmt.Fprintf(outputFile, "%s=%s\n", key, value)
	if err != nil {
		return fmt.Errorf("failed to write to env: %w", err)
	}

	return nil
}

func WriteApplicationDefaultCredentials(token, duration, projectNumber, poolId, providerId, serviceAccount string) (string, error) {
	// Define the file path
	tokenPath := filepath.Join("/tmp", "id_token.txt")

	// Write the token to the file
	if err := os.WriteFile(tokenPath, []byte(token), 0644); err != nil {
		return "", fmt.Errorf("failed to write token file: %w", err)
	}

	// Get the home directory path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Define the directory path
	dirPath := filepath.Join(homeDir, ".config", "gcloud")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Define the path to the credentials file
	filePath := filepath.Join(dirPath, "application_default_credentials.json")

	credentials := struct {
		Type                           string `json:"type"`
		Audience                       string `json:"audience"`
		SubjectTokenType               string `json:"subject_token_type"`
		TokenURL                       string `json:"token_url"`
		ServiceAccountImpersonationURL string `json:"service_account_impersonation_url"`
		CredentialSource               struct {
			File string `json:"file"`
		} `json:"credential_source"`
	}{
		Type:                           "external_account",
		Audience:                       fmt.Sprintf("https://iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s", projectNumber, poolId, providerId),
		SubjectTokenType:               "urn:ietf:params:oauth:token-type:id_token",
		TokenURL:                       "https://sts.googleapis.com/v1/token",
		ServiceAccountImpersonationURL: fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken", serviceAccount),
		CredentialSource: struct {
			File string `json:"file"`
		}{File: tokenPath},
	}

	credentialsJSON, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal credentials to JSON: %w", err)
	}

	// Write the credentials file
	if err := os.WriteFile(filePath, credentialsJSON, 0644); err != nil {
		return "", fmt.Errorf("failed to write credentials file: %w", err)
	}

	// Export credentials file to GOOGLE_APPLICATION_CREDENTIALS env
	// if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filePath); err != nil {
	// 	return "", fmt.Errorf("failed to set GOOGLE_APPLICATION_CREDENTIALS env variable: %w", err)
	// }

	if err := WriteEnvToFile("GOOGLE_APPLICATION_CREDENTIALS", filePath); err != nil {
		return "", fmt.Errorf("failed to write env to file: %w", err)
	}

	// log.Printf("GOOGLE_APPLICATION_CREDENTIALS is set to %s", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	logrus.Infof(fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS from env: %s", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))

	return filePath, nil
}
