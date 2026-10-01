package linux

import (
	"context"
	"fmt"
	"testing"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
)

type inventoryBenchmarkObserver struct {
	containers []dockerobs.ContainerSummary
	images     []dockerobs.ImageSummary
}

func (o *inventoryBenchmarkObserver) Observe(_ context.Context, request dockerobs.Request) (dockerobs.Response, error) {
	switch request.Operation {
	case dockerobs.OpContainerList:
		return dockerobs.Response{Containers: o.containers}, nil
	case dockerobs.OpImageList:
		return dockerobs.Response{Images: o.images}, nil
	default:
		return dockerobs.Response{Failed: true, Reason: "unsupported benchmark operation"}, nil
	}
}

func BenchmarkDockerInventory(b *testing.B) {
	observer := &inventoryBenchmarkObserver{
		containers: make([]dockerobs.ContainerSummary, 5000),
		images:     make([]dockerobs.ImageSummary, 5000),
	}
	for i := range observer.containers {
		id := fmt.Sprintf("%064x", i+1)
		image := "sha256:" + id
		observer.containers[i] = dockerobs.ContainerSummary{ID: id, Names: []string{fmt.Sprintf("svc-%d", i)}, ImageID: image}
		observer.images[i] = dockerobs.ImageSummary{ID: image}
	}
	c := collectorWith(b, []string{"containers", "images"}, nil, observer)
	for _, tc := range []struct {
		name string
		tool string
	}{
		{"containers", "list_docker_containers"},
		{"images", "list_docker_images"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result := c.Collect(context.Background(), tc.tool, contract.PageArgs{Limit: 100})
				if result.Error {
					b.Fatal(result.Issues)
				}
			}
		})
	}
}
