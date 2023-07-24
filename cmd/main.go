package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

func isImageRunningInCluster(contexts map[string][]string, imageToDelete ImageMetadata, repository string, registry string) error {
	for context, images := range contexts {
		for _, image := range images {
			for _, tag := range imageToDelete.Tags {
				if image == fmt.Sprintf("%s.azurecr.io/%s:%s", registry, repository, tag) {
					return fmt.Errorf("image with tag %s is running in the k8s context %s", tag, context)
				}
			}
		}
	}

	return nil
}

func parseKubectlContexts(k8sImages *[]string) error {
	output, err := exec.Command(
		"bash",
		"-c",
		"kubectl config get-contexts -o name",
	).Output()

	if err != nil {
		return err
	}

	for _, context := range strings.Split(string(output), "\n") {
		if len(context) <= 1 {
			continue
		}

		*k8sImages = append(*k8sImages, context)
	}

	return nil
}

func watchCmd(cmd string) error {
	azPurge := exec.Command("bash", "-c", fmt.Sprintf("%s", cmd))

	stdout, err := azPurge.StdoutPipe()
	if err != nil {
		return err
	}

	err = azPurge.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			log.Println(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}()

	err = azPurge.Wait()
	if err != nil {
		return err
	}

	return nil
}

func parseAgo(ago *string) (time.Time, error) {
	durationStr := (*ago)[:len(*ago)-1]
	durationType := string((*ago)[len(*ago)-1])

	durationInt, err := strconv.Atoi(durationStr)
	if err != nil {
		return time.Now(), fmt.Errorf("invalid duration number")
	}

	switch strings.ToLower(durationType) {
	case "s":
		return time.Now().Add(time.Duration(-durationInt) * time.Second), nil
	case "m":
		return time.Now().Add(time.Duration(-durationInt) * time.Minute), nil
	case "h":
		return time.Now().Add(time.Duration(-durationInt) * time.Hour), nil
	case "d":
		return time.Now().AddDate(0, 0, -durationInt), nil
	default:
		return time.Now(), fmt.Errorf("invalid duration type. Please use 's' for seconds, 'm' for minutes, 'h' for hours, or 'd' for days")
	}
}

func main() {
	registryName := flag.String("registry", "", "Name of the Azure Container Registry")
	repositoryName := flag.String("repository", "", "Name of the repository in your registry")
	subscriptionId := flag.String("subscription", "", "ID of the subscription. If not specified it will use the default one")
	contexts := flag.String("contexts", "", "Comma-separated list of Kubernetes contexts. The deletion process will not start if any 'imageToDelete' is running in a cluster from the context list")
	allContexts := flag.Bool("all-contexts", false, "The deletion process will not start if any 'imageToDelete' is running in a cluster from your kubeconfig contexts")
	ago := flag.String("ago", "360d", "Time duration in the past. Expects a number followed by a duration type: 's' for seconds, 'm' for minutes, 'h' for hours, 'd' for days.")
	dryRunMode := flag.Bool("dry-run", false, "Perform a dry run, print tags to be deleted but do not delete them")
	flag.Parse()

	if *repositoryName == "" || *registryName == "" {
		log.Println("You must provide the registry and repository")
		return
	}

	if *ago == "" {
		log.Println("You must provide a duration ago")
		return
	}

	if *subscriptionId != "" {
		_, err := exec.Command("bash", "-c", fmt.Sprintf("az account set --subscription %s", *subscriptionId)).Output()
		if err != nil {
			log.Println("Failed to set az subscription: ", err)
			log.Println("Are you logged in? (az login)")
			return
		}
	}

	providedContexts := make([]string, 0)
	if len(*contexts) > 0 {
		for _, context := range strings.Split(*contexts, ",") {
			if len(context) <= 1 {
				continue
			}

			providedContexts = append(providedContexts, strings.TrimSpace(context))
		}
	}

	if len(providedContexts) == 0 && *allContexts {
		err := parseKubectlContexts(&providedContexts)
		if err != nil {
			log.Fatalf("Error parsing all context from your kube config: %s", err)
		}
	}

	k8sImages := map[string][]string{}
	if len(providedContexts) > 0 {
		log.Printf("Parsing images from the following contexts: %s\n", strings.Join(providedContexts, ","))

		for _, context := range providedContexts {
			output, err := exec.Command(
				"bash",
				"-c",
				fmt.Sprintf("kubectl get pods --context %s --all-namespaces -o jsonpath=\"{.items[*].spec.containers[*].image}\"", context),
			).Output()
			if err != nil {
				log.Printf("Failed to get images from context: %s with error %s \n", context, err)
				continue
			}

			for _, image := range strings.Split(string(output), " ") {
				k8sImages[context] = append(k8sImages[context], image)
			}
		}
	}

	dateAgo, err := parseAgo(ago)
	if err != nil {
		log.Println("Unable to parse the provided ago timespan: ", err)
		return
	}

	listManifestsCmd := fmt.Sprintf(
		"az acr manifest list-metadata --name %s --registry %s --orderby time_asc --query \"[?lastUpdateTime < '%s']\"",
		*repositoryName, *registryName, dateAgo.Format(Layout),
	)
	manifestInformation, err := exec.Command("bash", "-c", listManifestsCmd).Output()
	if err != nil {
		log.Printf("Failed to retrieve manifest information from repository %s. Error: %s \n", *repositoryName, err)
		return
	}

	var imageMetadataList []ImageMetadata
	err = json.Unmarshal(manifestInformation, &imageMetadataList)
	if err != nil {
		log.Println("Error reading metadata: ", err)
		return
	}

	if len(imageMetadataList) == 0 {
		log.Printf("No Docker Images found which succeed the date %s\n", dateAgo)
		return
	}

	const bytesToGB = 1024 * 1024 * 1024
	totals := map[string]int{
		"images": 0,
		"bytes":  0,
	}
	for _, metadata := range imageMetadataList {
		if len(metadata.Tags) == 0 {
			continue
		}

		if len(k8sImages) != 0 {
			err = isImageRunningInCluster(k8sImages, metadata, *repositoryName, *registryName)
			if err != nil {
				log.Fatalf("Error: %s", err)
				return
			}
		}

		log.Printf("[DRY-RUN] Docker Image %s with tags %s would get deleted. Created Time: %s \n", *repositoryName, strings.Join(metadata.Tags, ","), metadata.CreatedTime)
		totals["images"]++
		totals["bytes"] += metadata.ImageSize
	}

	log.Printf("Found %d docker images with approximately %.2f GB worth of data to delete.", totals["images"], float64(totals["bytes"])/float64(bytesToGB))

	azPurgeCmd := fmt.Sprintf("az acr run --cmd=\"acr purge --filter '%s:.*' --ago=%s --untagged \" --registry %s /dev/null", *repositoryName, *ago, *registryName)
	if *dryRunMode {
		azPurgeCmd = fmt.Sprintf("az acr run --cmd=\"acr purge --filter '%s:.*' --ago=%s --untagged --dry-run\" --registry %s /dev/null", *repositoryName, *ago, *registryName)
	}

	log.Printf("Generated az purge cmd: %s", azPurgeCmd)
	log.Printf("Do you want to perfom the deletion? Please answer with yes")

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

	err = watchCmd(azPurgeCmd)
	if err != nil {
		log.Fatalf("Error fulfilling az purge command: %s", err)
	}
}
