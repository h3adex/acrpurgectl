package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/araddon/dateparse"
)

type ImageMetadata struct {
	Architecture         string `json:"architecture"`
	ChangeableAttributes struct {
		DeleteEnabled bool `json:"deleteEnabled"`
		ListEnabled   bool `json:"listEnabled"`
		ReadEnabled   bool `json:"readEnabled"`
		WriteEnabled  bool `json:"writeEnabled"`
	} `json:"changeableAttributes"`
	ConfigMediaType string    `json:"configMediaType"`
	CreatedTime     time.Time `json:"createdTime"`
	Digest          string    `json:"digest"`
	ImageSize       int       `json:"imageSize"`
	LastUpdateTime  time.Time `json:"lastUpdateTime"`
	MediaType       string    `json:"mediaType"`
	Os              string    `json:"os"`
	Tags            []string  `json:"tags"`
}

const Layout = "2006-01-02T15:04:05"

func isImageRunningInCluster(clusterImages []string, imageToDelete ImageMetadata, repository string) (string, error) {
	for _, clusterImage := range clusterImages {
		for _, tag := range imageToDelete.Tags {
			if strings.Contains(clusterImage, fmt.Sprintf("%s:%s", repository, tag)) {
				return clusterImage, fmt.Errorf("image is running in your provided k8s clusters")
			}
		}
	}

	return "", nil
}

func main() {
	registryName := flag.String("registry", "", "Name of the Azure Container Registry")
	repositoryName := flag.String("repository", "", "Name of the repository in your registry")
	subscriptionId := flag.String("subscription", "", "ID of the subscription. If not specified it will use the default one")
	contexts := flag.String("contexts", "", "Comma-separated list of Kubernetes contexts. The deletion process will not start if any 'imageToDelete' is running in a cluster from the context list")
	deletionCutoffTimestamp := flag.String("timestamp", "01/01/2024", "All Images before the timestamp will get deleted")
	delay := flag.Float64("delay", 1, "Delay between deletion requests")
	dryRunMode := flag.Bool("dry-run", false, "Perform a dry run, print tags to be deleted but do not delete them")
	flag.Parse()

	if *repositoryName == "" || *registryName == "" {
		log.Println("You must provide the registry and repository")
		return
	}

	if *subscriptionId != "" {
		_, err := exec.Command("bash", "-c", fmt.Sprintf("az account set --subscription %s", *subscriptionId)).Output()
		if err != nil {
			log.Println("Failed to set az subscription: ", err)
			return
		}
	}

	var k8sImages []string
	if len(*contexts) >= 0 {
		for _, context := range strings.Split(*contexts, ",") {
			output, err := exec.Command(
				"bash",
				"-c",
				fmt.Sprintf("kubectl get pods --context %s --all-namespaces -o jsonpath=\"{.items[*].spec.containers[*].image}\"", context),
			).Output()
			if err != nil {
				log.Println("Failed to set az subscription: ", err)
				return
			}

			for _, image := range strings.Split(string(output), " ") {
				k8sImages = append(k8sImages, image)
			}
		}
	}

	parsedDate, err := dateparse.ParseAny(*deletionCutoffTimestamp)
	if err != nil {
		log.Println("Unable to parse the provided date: ", err)
		return
	}

	listManifestsCmd := fmt.Sprintf(
		"az acr manifest list-metadata --name %s --registry %s --orderby time_asc --query \"[?lastUpdateTime < '%s']\"",
		*repositoryName, *registryName, parsedDate.Format(Layout),
	)

	manifestInformation, err := exec.Command("bash", "-c", listManifestsCmd).Output()
	if err != nil {
		log.Println("Failed to retrieve manifest information: ", err)
	}

	var imageMetadataList []ImageMetadata
	err = json.Unmarshal(manifestInformation, &imageMetadataList)
	if err != nil {
		log.Println("Error reading metadata: ", err)
		return
	}

	if len(imageMetadataList) == 0 {
		log.Printf("No Docker Images found which succeed the deletionCutoffTimestamp %s\n", parsedDate)
		return
	}

	var imagesToDelete []ImageMetadata
	for _, metadata := range imageMetadataList {
		if len(metadata.Tags) == 0 {
			continue
		}

		if len(k8sImages) != 0 {
			image, err := isImageRunningInCluster(k8sImages, metadata, *repositoryName)
			if err != nil {
				log.Fatalf("Error: The Image %s is running in one of your cluster. Please reconsider your deletion timestamp. \n", image)
			}
		}

		if *dryRunMode {
			log.Printf("[DRY-RUN] Docker Image %s with tags %s would get deleted. Created Time: %s \n", *repositoryName, strings.Join(metadata.Tags, ","), metadata.CreatedTime)
			continue
		}

		if len(metadata.Digest) > 0 {
			imagesToDelete = append(imagesToDelete, metadata)
		}
	}

	if len(imagesToDelete) == 0 {
		return
	}

	amountImages := 0
	for _, imageToDelete := range imagesToDelete {
		log.Printf("Docker Image %s with tags %s will get deleted. Created Time: %s \n", *repositoryName, strings.Join(imageToDelete.Tags, ","), imageToDelete.CreatedTime)
		amountImages++
	}

	log.Printf("%d Images will get deleted. Do you want to perfom the deletion? Please answer with yes\n", amountImages)

	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		log.Println("Unable to read user input")
		return
	}

	if response != "yes" {
		log.Println("Goodbye!")
		return
	}

	log.Printf("Starting deletion process with a delay of %f s \n", *delay)
	for _, imageToDelete := range imagesToDelete {
		if len(imageToDelete.Digest) == 0 {
			log.Printf("Skipping image with tags: %s since it has not digest \n", strings.Join(imageToDelete.Tags, ","))
		}

		deleteManifest := fmt.Sprintf(
			"az acr repository delete --name %s --image %s@%s --yes",
			*registryName, *repositoryName, imageToDelete.Digest,
		)
		_, err := exec.Command("bash", "-c", deleteManifest).Output()
		if err != nil {
			log.Printf("Error fulfilling deletion command: %s\n", err)
		}

		log.Printf("Deleted image %s with tags: %s \n", *repositoryName, strings.Join(imageToDelete.Tags, ","))
		time.Sleep(time.Second * time.Duration(*delay))
	}

	log.Printf("Done. Goodbye!")
}
