// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	aiplatform "cloud.google.com/go/aiplatform/apiv1beta1"
	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"google.golang.org/adk/v2/server/agentengine"

	util "github.com/tiaaburton/Data-Recon-Agent/internal/cliutil"
)

// envFileFlags carry configuration overrides
type envFileFlags struct {
	envFilePath    string
	apiKeySecret   string
	imageURI       string
	serviceAccount string
}

type gCloudFlags struct {
	region      string
	projectName string
}

type memoryBankFlags struct {
	deploy bool
	model  string
	ttl    time.Duration
}

type agentEngineServiceFlags struct {
	name          string
	displayName   string
	serverPort    int
	agentEngineID string
	memoryBank    memoryBankFlags
}

type buildFlags struct {
	tempDir             string
	execPath            string
	execFile            string
	dockerfileBuildPath string
	archivePath         string
}

type sourceFlags struct {
	srcBasePath        string
	entryPointPath     string
	origEntryPointPath string
	sourceDir          string
}

type deployAgentEngineFlags struct {
	gcloud      gCloudFlags
	agentEngine agentEngineServiceFlags
	build       buildFlags
	source      sourceFlags
	envFile     envFileFlags
}

var flags deployAgentEngineFlags

var agentEngineCmd = &cobra.Command{
	Use:   "agentengine",
	Short: "Deploys the application to Agent Engine.",
	Long:  `Deploys the application to Agent Engine. It creates a source archive, uploads it to create a Reasoning Engine, and cleans up temporary files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return flags.deployOnAgentEngine()
	},
}

func main() {
	if err := agentEngineCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	agentEngineCmd.PersistentFlags().StringVar(&flags.envFile.envFilePath, "env_file", "",
		"Path to a .env file whose KEY=VALUE lines become the deployment's environment.")
	agentEngineCmd.PersistentFlags().StringVar(&flags.envFile.imageURI, "image_uri", "",
		"Deploy a prebuilt container image instead of building from source.")
	agentEngineCmd.PersistentFlags().StringVar(&flags.envFile.serviceAccount, "service_account", "",
		"Run the engine as this service account instead of the default AI Platform service agent.")
	agentEngineCmd.PersistentFlags().StringVar(&flags.envFile.apiKeySecret, "api_key_secret", "",
		"Secret Manager secret to expose as GOOGLE_API_KEY. Empty means none (default for ADC).")

	agentEngineCmd.PersistentFlags().StringVarP(&flags.gcloud.region, "region", "r", "", "GCP Region")
	agentEngineCmd.PersistentFlags().StringVarP(&flags.gcloud.projectName, "project_name", "p", "", "GCP Project Name")
	agentEngineCmd.PersistentFlags().StringVarP(&flags.agentEngine.name, "name", "s", "", "Agent Engine name")
	agentEngineCmd.PersistentFlags().StringVarP(&flags.build.tempDir, "temp_dir", "t", "", "Temp dir for build, defaults to os.TempDir() if not specified")
	agentEngineCmd.PersistentFlags().IntVar(&flags.agentEngine.serverPort, "server_port", 8080, "agentEngine server port")
	agentEngineCmd.PersistentFlags().StringVarP(&flags.source.entryPointPath, "entry_point_path", "e", "", "Path to an entry point (go 'main')")
	agentEngineCmd.PersistentFlags().StringVarP(&flags.source.sourceDir, "source_dir", "d", "", "Directory to archive, defaults to current working directory")
	agentEngineCmd.PersistentFlags().StringVar(&flags.agentEngine.agentEngineID, "agent_engine_id", "", "ID of the Agent Engine instance to update if it exists.")
	agentEngineCmd.PersistentFlags().BoolVar(&flags.agentEngine.memoryBank.deploy, "mem_deploy", false, "If set to true then memory bank will be deployed too")
	agentEngineCmd.PersistentFlags().StringVar(&flags.agentEngine.memoryBank.model, "mem_model", "publishers/google/models/gemini-3.7-flash-preview", "Model for memory generation")
	agentEngineCmd.PersistentFlags().DurationVar(&flags.agentEngine.memoryBank.ttl, "mem_ttl", time.Hour*24*365, "Time-To-Live for memories")
}

func (f *deployAgentEngineFlags) computeFlags() error {
	return util.LogStartStop("Computing flags & preparing temp",
		func(p util.Printer) error {
			f.source.origEntryPointPath = flags.source.entryPointPath
			absp, err := filepath.Abs(flags.source.entryPointPath)
			if err != nil {
				return fmt.Errorf("cannot make an absolute path from '%v': %w", f.source.entryPointPath, err)
			}
			f.source.entryPointPath = absp

			if flags.build.tempDir == "" {
				flags.build.tempDir = os.TempDir()
			}
			absp, err = filepath.Abs(flags.build.tempDir)
			if err != nil {
				return fmt.Errorf("cannot make an absolute path from '%v': %w", f.build.tempDir, err)
			}
			f.build.tempDir, err = os.MkdirTemp(absp, "agentEngine_"+time.Now().Format("20060102_150405__")+"*")
			if err != nil {
				return fmt.Errorf("cannot create a temporary sub directory in '%v': %w", absp, err)
			}
			p("Using temp dir:", f.build.tempDir)

			dir, file := path.Split(f.source.entryPointPath)
			f.source.srcBasePath = dir
			f.source.entryPointPath = file
			if f.build.execPath == "" {
				exec, err := util.StripExtension(f.source.entryPointPath, ".go")
				if err != nil {
					return fmt.Errorf("cannot strip '.go' extension from entry point path '%v': %w", f.source.entryPointPath, err)
				}
				f.build.execFile = exec
				f.build.execPath = path.Join(f.build.tempDir, exec)
			}
			f.build.dockerfileBuildPath = path.Join(f.build.tempDir, "Dockerfile")
			f.build.archivePath = path.Join(f.build.tempDir, "archive.tgz")

			dateTimeString := time.Now().Format(time.RFC3339)
			f.agentEngine.displayName = f.agentEngine.name
			if f.agentEngine.displayName == "" {
				f.agentEngine.displayName = "ADK Agent: " + dateTimeString
			}

			f.agentEngine.memoryBank.model = fmt.Sprintf("projects/%s/locations/%s/%s", f.gcloud.projectName, f.gcloud.region, f.agentEngine.memoryBank.model)
			return nil
		})
}

func (f *deployAgentEngineFlags) cleanTemp() error {
	return util.LogStartStop("Cleaning temp",
		func(p util.Printer) error {
			p("Clean temp starting with", f.build.tempDir)
			err := os.RemoveAll(f.build.tempDir)
			if err != nil {
				return fmt.Errorf("failed to clean temp directory %v: %w", f.build.tempDir, err)
			}
			return nil
		})
}

func (f *deployAgentEngineFlags) prepareDockerfile() error {
	return util.LogStartStop("Preparing Dockerfile",
		func(p util.Printer) error {
			p("Writing:", f.build.dockerfileBuildPath)

			var b strings.Builder
			b.WriteString(`
FROM golang:alpine as builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ` + f.build.execFile + ` ` + f.source.origEntryPointPath + `

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/` + f.build.execFile + ` /app/` + f.build.execFile + `
COPY data/ ./data/
EXPOSE ` + strconv.Itoa(flags.agentEngine.serverPort) + `
CMD ["/app/` + f.build.execFile + `", "web", "-port", "` + strconv.Itoa(flags.agentEngine.serverPort) + `", "agentengine"]`)
			return os.WriteFile(f.build.dockerfileBuildPath, []byte(b.String()), 0o600)
		})
}

func (f *deployAgentEngineFlags) createArchive() error {
	return util.LogStartStop("Creating source archive",
		func(p util.Printer) error {
			workspaceRoot := f.source.sourceDir
			if workspaceRoot == "" {
				var err error
				workspaceRoot, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("cannot get current working directory: %w", err)
				}
			}
			p("Creating:", f.build.archivePath)
			cmd := exec.Command("tar", "-czf", f.build.archivePath,
				"-C", workspaceRoot, "--exclude=.git", "--exclude=bin", "--exclude=adkgo", "--exclude=terraform", "--exclude=.terraform", "--exclude=tmp", ".",
				"-C", f.build.tempDir, "Dockerfile")
			return util.LogCommand(cmd, p)
		})
}

func (f *deployAgentEngineFlags) gcloudDeployToAgentEngine() error {
	return util.LogStartStop("Deploying to Agent Engine",
		func(p util.Printer) error {
			ctx := context.Background()
			parent := fmt.Sprintf("projects/%s/locations/%s", f.gcloud.projectName, f.gcloud.region)
			endpoint := fmt.Sprintf("%s-aiplatform.googleapis.com:443", f.gcloud.region)
			client, err := aiplatform.NewReasoningEngineClient(ctx, option.WithEndpoint(endpoint))
			if err != nil {
				return fmt.Errorf("cannot create ReasoningEngineClient: %w", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					p("Warning: failed to close ReasoningEngineClient: %v", err)
				}
			}()

			archiveContent, err := os.ReadFile(f.build.archivePath)
			if err != nil {
				return fmt.Errorf("cannot read archive file: %w", err)
			}

			methods, err := agentengine.ListClassMethods()
			if err != nil {
				return fmt.Errorf("cannot list class methods: %w", err)
			}
			methodsJSON, err := json.Marshal(methods)
			if err != nil {
				return fmt.Errorf("cannot marshal methods: %w", err)
			}
			p("Methods:", string(methodsJSON))

			req := &aiplatformpb.CreateReasoningEngineRequest{
				Parent: parent,
				ReasoningEngine: &aiplatformpb.ReasoningEngine{
					DisplayName: f.agentEngine.displayName,
					Spec: &aiplatformpb.ReasoningEngineSpec{
						AgentFramework: "google-adk",
						DeploymentSpec: f.deploymentSpec(),
						ClassMethods:   methods,
					},
				},
			}
			f.setDeploymentSource(req.ReasoningEngine.Spec, archiveContent)

			if f.agentEngine.memoryBank.deploy {
				req.ReasoningEngine.ContextSpec = &aiplatformpb.ReasoningEngineContextSpec{
					MemoryBankConfig: &aiplatformpb.ReasoningEngineContextSpec_MemoryBankConfig{
						GenerationConfig: &aiplatformpb.ReasoningEngineContextSpec_MemoryBankConfig_GenerationConfig{
							Model: f.agentEngine.memoryBank.model,
						},
						TtlConfig: &aiplatformpb.ReasoningEngineContextSpec_MemoryBankConfig_TtlConfig{
							Ttl: &aiplatformpb.ReasoningEngineContextSpec_MemoryBankConfig_TtlConfig_DefaultTtl{
								DefaultTtl: durationpb.New(f.agentEngine.memoryBank.ttl),
							},
						},
					},
				}
			}

			p("Sending CreateReasoningEngine request...")
			op, err := client.CreateReasoningEngine(ctx, req)
			if err != nil {
				return fmt.Errorf("CreateReasoningEngine failed: %w", err)
			}

			p("Waiting for operation to complete...")
			re, err := op.Wait(ctx)
			if err != nil {
				return fmt.Errorf("operation failed: %w", err)
			}

			p("Deployed Reasoning Engine:", re.Name)
			p("Display Name:", re.DisplayName)

			return nil
		})
}

func (f *deployAgentEngineFlags) gcloudUpdateAgentEngine() error {
	return util.LogStartStop("Updating Agent Engine",
		func(p util.Printer) error {
			ctx := context.Background()
			name := fmt.Sprintf("projects/%s/locations/%s/reasoningEngines/%s", f.gcloud.projectName, f.gcloud.region, f.agentEngine.agentEngineID)
			endpoint := fmt.Sprintf("%s-aiplatform.googleapis.com:443", f.gcloud.region)
			client, err := aiplatform.NewReasoningEngineClient(ctx, option.WithEndpoint(endpoint))
			if err != nil {
				return fmt.Errorf("cannot create ReasoningEngineClient: %w", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					p("Warning: failed to close ReasoningEngineClient: %v", err)
				}
			}()

			archiveContent, err := os.ReadFile(f.build.archivePath)
			if err != nil {
				return fmt.Errorf("cannot read archive file: %w", err)
			}

			methods, err := agentengine.ListClassMethods()
			if err != nil {
				return fmt.Errorf("cannot list class methods: %w", err)
			}
			methodsJSON, err := json.Marshal(methods)
			if err != nil {
				return fmt.Errorf("cannot marshal methods: %w", err)
			}
			p("Methods:", string(methodsJSON))

			req := &aiplatformpb.UpdateReasoningEngineRequest{
				ReasoningEngine: &aiplatformpb.ReasoningEngine{
					Name: name,
					Spec: &aiplatformpb.ReasoningEngineSpec{
						AgentFramework: "google-adk",
						DeploymentSpec: f.deploymentSpec(),
						ClassMethods:   methods,
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: f.updateMaskPaths()},
			}
			f.setDeploymentSource(req.ReasoningEngine.Spec, archiveContent)

			p("Sending UpdateReasoningEngine request...")
			op, err := client.UpdateReasoningEngine(ctx, req)
			if err != nil {
				return fmt.Errorf("UpdateReasoningEngine failed: %w", err)
			}

			p("Waiting for operation to complete...")
			re, err := op.Wait(ctx)
			if err != nil {
				return fmt.Errorf("operation failed: %w", err)
			}

			p("Updated Reasoning Engine:", re.Name)
			p("Display Name:", re.DisplayName)

			return nil
		})
}

func (f *deployAgentEngineFlags) deployOnAgentEngine() error {
	err := f.computeFlags()
	if err != nil {
		return err
	}
	err = f.prepareDockerfile()
	if err != nil {
		return err
	}
	err = f.createArchive()
	if err != nil {
		return err
	}
	if f.agentEngine.agentEngineID != "" {
		err = f.gcloudUpdateAgentEngine()
	} else {
		err = f.gcloudDeployToAgentEngine()
	}
	if err != nil {
		return err
	}
	err = f.cleanTemp()
	if err != nil {
		return err
	}

	return nil
}

func (f *deployAgentEngineFlags) deploymentSpec() *aiplatformpb.ReasoningEngineSpec_DeploymentSpec {
	env := []*aiplatformpb.EnvVar{
		{Name: "GOOGLE_CLOUD_REGION", Value: f.gcloud.region},
		{Name: "NUM_WORKERS", Value: "1"},
		{Name: "GOOGLE_CLOUD_AGENT_ENGINE_ENABLE_TELEMETRY", Value: "true"},
		{Name: "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", Value: "true"},
	}

	seen := map[string]bool{}
	for _, e := range env {
		seen[e.Name] = true
	}
	for _, kv := range loadEnvFile(f.envFile.envFilePath) {
		if seen[kv.Name] {
			continue
		}
		if reservedEnvNames[kv.Name] {
			continue
		}
		seen[kv.Name] = true
		env = append(env, kv)
	}

	spec := &aiplatformpb.ReasoningEngineSpec_DeploymentSpec{Env: env}

	if f.envFile.apiKeySecret != "" {
		spec.SecretEnv = []*aiplatformpb.SecretEnvVar{{
			Name:      "GOOGLE_API_KEY",
			SecretRef: &aiplatformpb.SecretRef{Secret: f.envFile.apiKeySecret, Version: "latest"},
		}}
	}
	return spec
}

var reservedEnvNames = map[string]bool{
	"GOOGLE_CLOUD_PROJECT":           true,
	"GOOGLE_CLOUD_LOCATION":          true,
	"GOOGLE_CLOUD_REGION":            true,
	"GOOGLE_APPLICATION_CREDENTIALS": true,
	"PORT":                           true,
	"K_SERVICE":                      true,
	"K_REVISION":                     true,
	"K_CONFIGURATION":                true,
}

func loadEnvFile(path string) []*aiplatformpb.EnvVar {
	if path == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var out []*aiplatformpb.EnvVar
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		trimmedVal := strings.TrimSpace(value)
		if !strings.HasPrefix(trimmedVal, `"`) && !strings.HasPrefix(trimmedVal, `'`) {
			if idx := strings.Index(value, "#"); idx >= 0 {
				value = value[:idx]
			}
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || value == "" {
			continue
		}
		out = append(out, &aiplatformpb.EnvVar{Name: key, Value: value})
	}
	return out
}

func (f *deployAgentEngineFlags) setDeploymentSource(spec *aiplatformpb.ReasoningEngineSpec, archive []byte) {
	if f.envFile.serviceAccount != "" {
		spec.ServiceAccount = &f.envFile.serviceAccount
	}
	if f.envFile.imageURI != "" {
		spec.DeploymentSource = &aiplatformpb.ReasoningEngineSpec_ContainerSpec_{
			ContainerSpec: &aiplatformpb.ReasoningEngineSpec_ContainerSpec{
				ImageUri: f.envFile.imageURI,
			},
		}
		return
	}
	spec.DeploymentSource = &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec_{
		SourceCodeSpec: &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec{
			Source: &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec_InlineSource_{
				InlineSource: &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec_InlineSource{
					SourceArchive: archive,
				},
			},
			LanguageSpec: &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec_ImageSpec_{
				ImageSpec: &aiplatformpb.ReasoningEngineSpec_SourceCodeSpec_ImageSpec{},
			},
		},
	}
}

func (f *deployAgentEngineFlags) updateMaskPaths() []string {
	source := "spec.source_code_spec"
	if f.envFile.imageURI != "" {
		source = "spec.container_spec"
	}
	paths := []string{source, "spec.class_methods", "spec.deployment_spec"}
	if f.envFile.serviceAccount != "" {
		paths = append(paths, "spec.service_account")
	}
	return paths
}
