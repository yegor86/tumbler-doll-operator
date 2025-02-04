package workflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
)

var (
	dockerClientVersion string = "1.46"
)

type DockerContainer struct {
	DockerClient *dockerClient.Client
	ContainerId  string
}

// NewDockerContainer starts a Docker container with a specified image
func NewDockerContainer(ctx context.Context, imageName string) (*DockerContainer, error) {

	// Create a Docker client
	docker, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithVersion(dockerClientVersion))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Pull the image (if not present locally)
	pullOut, err := docker.ImagePull(ctx, buildImageWithTag(imageName), image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull Docker image: %w", err)
	}
	defer pullOut.Close()
	//TODO: Stream docker pull output on to the progress view
	io.Copy(io.Discard, pullOut)

	// Create the container
	resp, err := docker.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Entrypoint: []string{"sh"},
		Tty:   true,
	}, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker container: %w", err)
	}

	containerId := resp.ID

	// Start the container
	if err := docker.ContainerStart(ctx, containerId, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start Docker container: %w", err)
	}

	return &DockerContainer{
		DockerClient: docker,
		ContainerId:  containerId,
	}, nil
}

func (dm *DockerContainer) StopContainer(ctx context.Context, imageName string) error {
	// Stop and remove the container after all commands are executed
	if err := dm.DockerClient.ContainerStop(ctx, dm.ContainerId, container.StopOptions{}); err != nil {
		return err
	}
	if err := dm.DockerClient.ContainerRemove(ctx, dm.ContainerId, container.RemoveOptions{}); err != nil {
		return err
	}
	if err := dm.DockerClient.Close(); err != nil {
		return err
	}
	return nil
}

func Containerize(next func(params map[string]interface{}) string) func(params map[string]interface{}) string {
	return func(params map[string]interface{}) string {
		if containerId, ok := params["containerId"]; ok {
			text := params["text"].(string)
			terms := strings.Fields(text)
			output, err := ExecContainer(context.Background(), containerId.(string), terms)
			
			if err != nil {
				return fmt.Errorf("error attaching to container %s: %v", containerId, err).Error()
			}
			return string(output)
		} else {
    		return next(params)
		}
    }
}

func ExecContainer(ctx context.Context, containerId string, cmd []string) ([]byte, error) {
	// Create a Docker client
	docker, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithVersion(dockerClientVersion))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer docker.Close()

	execResp, err := docker.ContainerExecCreate(ctx, containerId, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create exec instance for command '%v': %w", cmd, err)
	}

	// Start the command execution
	execAttachResp, err := docker.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start exec instance for command '%v': %w", cmd, err)
	}
	defer execAttachResp.Close()
	execAttachResp.CloseWrite()

	// Capture the output
	output, err := io.ReadAll(execAttachResp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read output for command '%v': %w", cmd, err)
	}
	return removeControlChars(output), nil
}

// Append tag 'latest' to image without tag
func buildImageWithTag(imageName string) string {
	imageTag := strings.Split(imageName, ":")
	if len(imageTag) > 1 {
		return imageTag[0] + ":" + imageTag[1]
	}
	return imageTag[0] + ":latest"
}

// RemoveControlChars removes non-printable ASCII characters from byte array and return human readble string.
func removeControlChars(input []byte) []byte {
    return bytes.Map(func(r rune) rune {
        if unicode.IsControl(r) {
            return -1
        }
        return r
    }, input)
}
