package workflow

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/strslice"
	docker_client "github.com/docker/docker/client"
	"go.temporal.io/sdk/activity"
)

type SampleActivities struct {
}

func (a *SampleActivities) SampleActivity(ctx context.Context, commands []string, containerImage string) ([]string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	fmt.Printf("Run %s with command %v \n", name, commands)
	
	if containerImage != "" {
		return DockerActivity(ctx, containerImage, commands)
	}

	var results []string
	for _, command := range commands {
		cmd := exec.Command("bash", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			log.Printf("Command execution failed: %s", err)
			return results, err
		}
		fmt.Printf("Command Output: %s\n", output)
	}

	return results, nil
}

// DockerActivity starts a Docker container with a specified image
func DockerActivity(ctx context.Context, imageName string, commands []string) ([]string, error) {
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    // defer cancel()

	// Create a Docker client
	cli, err := docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithVersion("1.46"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Pull the image (if not present locally)
	_, err = cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull Docker image: %w", err)
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Tty:   true,
	}, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker container: %w", err)
	}

	containerID := resp.ID

	// Start the container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start Docker container: %w", err)
	}

	// Execute each command separately inside the container
	results := []string{}
	for _, cmd := range commands {
		execResp, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
			Cmd:          strslice.StrSlice{"sh", "-c", cmd},
			AttachStdout: true,
			AttachStderr: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create exec instance for command '%s': %w", cmd, err)
		}

		// Start the command execution
		execStartResp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to start exec instance for command '%s': %w", cmd, err)
		}

		// Capture the output
		output, err := io.ReadAll(execStartResp.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read output for command '%s': %w", cmd, err)
		}
		results = append(results, string(output))
	}

	// Stop and remove the container after all commands are executed
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}
	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		log.Printf("Failed to remove container: %v", err)
	}

	return results, nil
}
