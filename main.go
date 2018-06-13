// Craig Hesling
// June 13, 2018
//
// This is an OpenChirp service that evaluates a logical or mathematical
// expression upon receiving updates to transducers referenced in the expression.
// It should be noted that this service assumes that transducers are only
// one level deep. This is because we must use them as variable names in an
// expressions. Since "/" maps to divide, we cannot express hierarchical
// transducer names.
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/Knetic/govaluate"

	"github.com/openchirp/framework"
	"github.com/openchirp/framework/utils"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	version string = "1.0"
)

const (
	configKeyExpressions  = "Expressions"
	configKeyOutputTopics = "Output Topics"
	configKeyOptions      = "Options"
	optionBoolAsValue     = "boolasvalue"
)

const (
	defaultOutputTopicPrefix = "expr"
	errorTopic               = framework.TransducerPrefix + "/math-evaluate-error"
)

const (
	// Set this value to true to have the service publish a service status of
	// "Running" each time it receives a device update event
	runningStatus = true
)

// Device holds the device specific last values and target topics for the difference.
type Device struct {
	expressions []*govaluate.EvaluableExpression
	outtopics   []string
	lastvalues  map[string]interface{}
	boolAsValue bool
}

// NewDevice is called by the framework when a new device has been linked.
func NewDevice() framework.Device {
	d := new(Device)
	return d
}

// ProcessLink is called once, during the initial setup of a
// device, and is provided the service config for the linking device.
func (d *Device) ProcessLink(ctrl *framework.DeviceControl) string {
	logitem := log.WithField("deviceid", ctrl.Id())
	logitem.Info("Linking with config:", ctrl.Config())

	exprs, err := utils.ParseCSVConfig(ctrl.Config()[configKeyExpressions])
	if err != nil {
		var ret string
		if e, ok := err.(*csv.ParseError); ok {
			ret = fmt.Sprintf("Error parsing %s at column %d: %v", configKeyExpressions, e.Column, e.Err)
		} else {
			ret = fmt.Sprintf("Error parsing %s: %v", configKeyExpressions, err.Error())
		}
		logitem.Info(ret)
		return ret
	}

	topics, err := utils.ParseCSVConfig(ctrl.Config()[configKeyOutputTopics])
	if err != nil {
		var ret string
		if e, ok := err.(*csv.ParseError); ok {
			ret = fmt.Sprintf("Error parsing %s at column %d: %v", configKeyOutputTopics, e.Column, e.Err)
		} else {
			ret = fmt.Sprintf("Error parsing %s: %v", configKeyOutputTopics, err.Error())
		}
		logitem.Info(ret)
		return err.Error()
	}

	options, err := utils.ParseCSVConfig(ctrl.Config()[configKeyOptions])
	if err != nil {
		var ret string
		if e, ok := err.(*csv.ParseError); ok {
			ret = fmt.Sprintf("Error parsing %s at column %d: %v", configKeyOptions, e.Column, e.Err)
		} else {
			ret = fmt.Sprintf("Error parsing %s: %v", configKeyOptions, err.Error())
		}
		logitem.Info(ret)
		return ret
	}

	d.boolAsValue = false
	for _, option := range options {
		if strings.ToLower(option) == optionBoolAsValue {
			d.boolAsValue = true
		}
	}

	d.expressions = make([]*govaluate.EvaluableExpression, len(exprs))
	d.outtopics = make([]string, len(exprs))
	d.lastvalues = make(map[string]interface{})

	for i := range exprs {
		expr, err := govaluate.NewEvaluableExpression(exprs[i])
		if err != nil {
			return fmt.Sprintf("Error parsing expression #%d \"%s\"", i+1, exprs[i])
		}
		d.expressions[i] = expr

		if (i < len(topics)) && (topics[i] != "") {
			d.outtopics[i] = topics[i]
		} else {
			d.outtopics[i] = fmt.Sprintf("%s%d", defaultOutputTopicPrefix, i)
		}
	}
	logitem.Debug("Parsed expressions and output topics")

	// Reverse dependency list:
	//  Transducer name to the expression+output-topic indicies they would
	//  change, if itself was updated.
	transducerToIndex := make(map[string][]int)
	for i, e := range d.expressions {
		v := e.Vars()
		sort.Strings(v)
		var last string
		for _, s := range v {
			if last != s {
				indicies, ok := transducerToIndex[s]
				if !ok {
					indicies = make([]int, 0)
				}
				indicies = append(indicies, i)
				transducerToIndex[s] = indicies
			}
			last = s
		}
	}
	logitem.Debug("Built reverse transducer name to dependent indicies map")

	for transducerName, indicies := range transducerToIndex {
		topic := framework.TransducerPrefix + "/" + transducerName
		ctrl.Subscribe(topic, indicies)
		logitem.Debug("Subscribing to transducer ", topic, ", which references indicies ", indicies)

		// Also subscribe to the "-" variant of the transducer topic
		if strings.ContainsRune(transducerName, '_') {
			topic := framework.TransducerPrefix + "/" + strings.Replace(transducerName, "_", "-", -1)
			ctrl.Subscribe(topic, indicies)
			logitem.Debug("Subscribing to transducer ", topic, ", which references indicies ", indicies)
		}
	}

	logitem.Info("Finished Linking Successfully")

	// This message is sent to the service status for the linking device
	return "Success"
}

// ProcessUnlink is called once, when the service has been unlinked from
// the device.
func (d *Device) ProcessUnlink(ctrl *framework.DeviceControl) {
	logitem := log.WithField("deviceid", ctrl.Id())
	d.expressions = nil
	d.outtopics = nil
	d.lastvalues = nil
	logitem.Info("Unlinked:")
}

// ProcessConfigChange is ignored in this case.
func (d *Device) ProcessConfigChange(ctrl *framework.DeviceControl, cchanges, coriginal map[string]string) (string, bool) {
	logitem := log.WithField("deviceid", ctrl.Id())

	logitem.Info("Ignoring Config Change:", cchanges)
	return "", false
}

// ProcessMessage is called upon receiving a pubsub message destined for
// this device.
func (d *Device) ProcessMessage(ctrl *framework.DeviceControl, msg framework.Message) {
	logitem := log.WithField("deviceid", ctrl.Id())
	logitem.Debugf("Processing expression event for topic %s", msg.Topic())

	value := utils.ParseOCValue(string(msg.Payload()))

	// First value is only stored, so that we don't get spurious spikes
	topicParts := strings.Split(msg.Topic(), "/")
	transducerName := topicParts[len(topicParts)-1]
	transducerName = strings.Replace(transducerName, "-", "_", -1)
	if d.lastvalues[transducerName] == nil {
		logitem.Infof("Setting first value for \"%s\" | value=%v", transducerName, value)
	}
	d.lastvalues[transducerName] = value

	// Check all expressions that may have been impacted
	for _, index := range msg.Key().([]int) {
		e := d.expressions[index]
		result, err := e.Evaluate(d.lastvalues)
		if err != nil {
			// We are probably missing a parameter
			logitem.Debugf("Evaluation of \"%s\" resulted in err: %v", e.String(), err)
			ctrl.Publish(errorTopic, err.Error())
			continue
		}
		var value string = fmt.Sprint(result)
		switch result.(type) {
		case float64:
			value = fmt.Sprintf("%.10f", result.(float64))
		case bool:
			if d.boolAsValue {
				if result.(bool) {
					value = "1"
				} else {
					value = "0"
				}
			}
		}
		topic := framework.TransducerPrefix + "/" + d.outtopics[index]
		ctrl.Publish(topic, value)
		logitem.Debugf("Published value %v to %s", value, topic)
	}
}

// run is the main function that gets called once form main()
func run(ctx *cli.Context) error {
	/* Set logging level (verbosity) */
	log.SetLevel(log.Level(uint32(ctx.Int("log-level"))))

	log.Info("Starting Math Evaluate Service")

	/* Start framework service client */
	c, err := framework.StartServiceClientManaged(
		ctx.String("framework-server"),
		ctx.String("mqtt-server"),
		ctx.String("service-id"),
		ctx.String("service-token"),
		"Unexpected disconnect!",
		NewDevice)
	if err != nil {
		log.Error("Failed to StartServiceClient: ", err)
		return cli.NewExitError(nil, 1)
	}
	defer c.StopClient()
	log.Info("Started service")

	/* Post service's global status */
	if err := c.SetStatus("Starting"); err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Info("Published Service Status")

	/* Setup signal channel */
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	/* Post service status indicating I started */
	if err := c.SetStatus("Started"); err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Info("Published Service Status")

	/* Wait on a signal */
	sig := <-signals
	log.Info("Received signal ", sig)
	log.Warning("Shutting down")

	/* Post service's global status */
	if err := c.SetStatus("Shutting down"); err != nil {
		log.Error("Failed to publish service status: ", err)
	}
	log.Info("Published service status")

	return nil
}

func main() {
	/* Parse arguments and environmental variable */
	app := cli.NewApp()
	app.Name = "math-evaluate-service"
	app.Usage = ""
	app.Copyright = "See https://github.com/openchirp/math-evaluate-service for copyright information"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "framework-server",
			Usage:  "OpenChirp framework server's URI",
			Value:  "http://localhost:7000",
			EnvVar: "FRAMEWORK_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "MQTT server's URI (e.g. scheme://host:port where scheme is tcp or tls)",
			Value:  "tls://localhost:1883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "service-id",
			Usage:  "OpenChirp service id",
			EnvVar: "SERVICE_ID",
		},
		cli.StringFlag{
			Name:   "service-token",
			Usage:  "OpenChirp service token",
			EnvVar: "SERVICE_TOKEN",
		},
		cli.IntFlag{
			Name:   "log-level",
			Value:  4,
			Usage:  "debug=5, info=4, warning=3, error=2, fatal=1, panic=0",
			EnvVar: "LOG_LEVEL",
		},
	}

	/* Launch the application */
	app.Run(os.Args)
}
