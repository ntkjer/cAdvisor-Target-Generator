package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gopkg.in/yaml.v2"
)

const (
	_cAdvisorPort    = "8080"
	_configPath      = "./"
	_configName      = "prometheus.yml"
	checkoutClientJC = "checkout-client"
	checkoutAPI      = "checkout-api"
	scNextJC         = "sidecar-next"
	scNextFC         = "sidecar-next-fc"
	checkoutClientFC = "checkout-client-fc"
	cmsBackend       = "cms-backend"
)

var buffer map[string][]string
var cmsBackendIPs []string
var checkoutClientJcrewIPs []string
var checkoutClientFactoryIPs []string
var checkoutGraphQLIPs []string
var scNextJcrewIPs []string
var scNextFactoryIPs []string
var config string

//Creates an AWS client session
func createSession() *ec2.DescribeInstancesOutput {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create new EC2 client
	svc := ec2.New(sess)

	// Create input filter for AWS query
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String("*"),
				},
			},
		},
	}
	resp, err := svc.DescribeInstances(input)
	checkFatal(err)
	return resp
}

func parseDataAWS(response *ec2.DescribeInstancesOutput) map[string][]string {

	buffer = make(map[string][]string)

	for id := range response.Reservations {
		for _, instance := range response.Reservations[id].Instances {
			for _, item := range instance.Tags {
				currKey := *item.Key
				currVal := *item.Value
				if currKey == "ApplicationName" {
					if strings.Contains(currVal, checkoutClientJC) {
						checkoutClientJcrewIPs = append(checkoutClientJcrewIPs, *instance.PrivateIpAddress)
					} else if strings.Contains(currVal, checkoutAPI) {
						checkoutGraphQLIPs = append(checkoutGraphQLIPs, *instance.PrivateIpAddress)
					} else if strings.Contains(currVal, scNextJC) {
						scNextJcrewIPs = append(scNextJcrewIPs, *instance.PrivateIpAddress)
					} else if strings.Contains(currVal, scNextFC) {
						scNextFactoryIPs = append(scNextFactoryIPs, *instance.PrivateIpAddress)
					} else if strings.Contains(currVal, cmsBackend) {
						cmsBackendIPs = append(cmsBackendIPs, *instance.PrivateIpAddress)
					} else if strings.Contains(currVal, checkoutClientFC) {
						checkoutClientFactoryIPs = append(checkoutClientFactoryIPs, *instance.PrivateIpAddress)
					}
				}
			}
		}
	}
	buffer[checkoutClientJC] = checkoutClientJcrewIPs
	buffer[checkoutClientFC] = checkoutClientFactoryIPs
	buffer[checkoutAPI] = checkoutGraphQLIPs
	buffer[scNextFC] = scNextFactoryIPs
	buffer[scNextJC] = scNextJcrewIPs
	buffer[cmsBackend] = cmsBackendIPs
	return buffer
}

func main() {
	resp := createSession()
	data := parseDataAWS(resp)
	config = generatePromConfig(data)
	yamlWriter(config)
	lintTargets()
}

/*
 */
type PrometheusConfig struct {
	//Anonymous type/struct inference
	Global        Global          `yaml:"global"`
	ScrapeConfigs []ScrapeConfigs `yaml:"scrape_configs"`
}

/*
Global:
*/
type Global struct {
	ScrapeInterval string    `yaml:"scrape_interval"`
	ExtLabels      ExtLabels `yaml:"external_labels"`
}

/*
ExtLabels
This does some shit that returns param1, param2
*/
type ExtLabels struct {
	Monitor string `yaml:"monitor"`
}

/*
This does some shit that returns param1, param2
*/
type ScrapeConfigs struct {
	JobName        string        `yaml:"job_name"`
	ScrapeInterval string        `yaml:"scrape_interval,omitempty"`
	StaticConfigs  StaticConfigs `yaml:"static_configs"`
}

/*
This does some shit that returns param1, param2
*/
type StaticConfigs struct {
	Targets []string `yaml:"targets,flow"`
}

func appendPort(ip string) *string {
	var buffer bytes.Buffer
	var result string
	buffer.WriteString(ip)
	buffer.WriteString(":")
	buffer.WriteString(_cAdvisorPort)
	result = buffer.String()
	return &result
}

/*
Generates a prometheus config using the defined structs and outputs a string representation of it
*/
func generatePromConfig(buffer map[string][]string) string {
	var extLabel ExtLabels
	var globalLabel = os.Getenv("AWS_ACCOUNT")
	var labelHandle = `monitor: ` + globalLabel
	yaml.Unmarshal([]byte(labelHandle), &extLabel)
	var global Global
	global.ScrapeInterval = "15s"
	global.ExtLabels = extLabel
	var scrapeCollection []ScrapeConfigs
	var baseTargetHandler = `targets: [`
	for key, value := range buffer {
		var targetHandler = baseTargetHandler
		var statConfig StaticConfigs
		for index, ip := range value {
			var buffer bytes.Buffer
			var result string
			buffer.WriteString("'")
			buffer.WriteString(ip)
			buffer.WriteString(":")
			buffer.WriteString(_cAdvisorPort)
			buffer.WriteString("'")
			result = buffer.String()
			targetHandler = targetHandler + result
			if index < len(value)-1 {
				targetHandler = targetHandler + ","
			} else {
				targetHandler = targetHandler + "]"
			}
		}
		//Popualte Static config
		statErr := yaml.Unmarshal([]byte(targetHandler), &statConfig)
		check(statErr)

		var scraper ScrapeConfigs
		scraper.JobName = key
		scraper.ScrapeInterval = "10s"
		scraper.StaticConfigs = statConfig
		scrapeCollection = append(scrapeCollection, scraper)

		//Reset our localized staticConfiguration struct for the next iteration
		statConfig = StaticConfigs{}
	}

	var promConfig PrometheusConfig
	promConfig.Global = global
	promConfig.ScrapeConfigs = scrapeCollection

	d, err := yaml.Marshal(&promConfig)
	checkFatal(err)
	resp := string(d)
	return resp
}

func check(e error) {
	if e != nil {
		log.Println(e)
	}
}

func checkFatal(e error) {
	if e != nil {
		panic(e)
	}
}

func yamlWriter(conf string) {
	var currentConfig = _configPath + _configName
	err := ioutil.WriteFile(currentConfig, []byte(conf), 0644)
	checkFatal(err)
	fmt.Println("Success!")
	fmt.Printf("Generated file: %s\n", currentConfig)
}

//Reads config file
func lintTargets() {
	_, err := exec.Command("sed", "-i", "s/targets/- targets/g", "./prometheus.yml").Output()
	checkFatal(err)
}
