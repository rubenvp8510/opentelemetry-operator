package config

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// ErrInvalidYAML represents an error in the format of the original YAML configuration file.
var (
	ErrInvalidYAML = errors.New("couldn't parse the loadbalancer configuration")
)

const DefaultResyncTime = 5 * time.Minute
const DefaultConfigFilePath string = "/conf/targetallocator.yaml"

type Config struct {
	LabelSelector map[string]string  `yaml:"label_selector,omitempty"`
	Config        *promconfig.Config `yaml:"config"`
}

type PrometheusCRWatcherConfig struct {
	Enabled *bool
}

type CLIConfig struct {
	ListenAddr     *string
	ConfigFilePath *string
	ClusterConfig  *rest.Config
	// KubeConfigFilePath empty if in cluster configuration is in use
	KubeConfigFilePath string
	RootLogger         logr.Logger
	PromCRWatcherConf  PrometheusCRWatcherConfig
}

func Load(file string) (Config, error) {
	var cfg Config
	if err := unmarshal(&cfg, file); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func unmarshal(cfg *Config, configFile string) error {

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	if err = yaml.UnmarshalStrict(yamlFile, cfg); err != nil {
		return fmt.Errorf("error unmarshaling YAML: %w", err)
	}
	return nil
}

func ParseCLI() (CLIConfig, error) {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	cLIConf := CLIConfig{
		ListenAddr:     pflag.String("listen-addr", ":8080", "The address where this service serves."),
		ConfigFilePath: pflag.String("config-file", DefaultConfigFilePath, "The path to the config file."),
		PromCRWatcherConf: PrometheusCRWatcherConfig{
			Enabled: pflag.Bool("enable-prometheus-cr-watcher", false, "Enable Prometheus CRs as target sources"),
		},
	}
	kubeconfigPath := pflag.String("kubeconfig-path", filepath.Join(homedir.HomeDir(), ".kube", "config"), "absolute path to the KubeconfigPath file")
	pflag.Parse()

	cLIConf.RootLogger = zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(cLIConf.RootLogger)
	ctrl.SetLogger(cLIConf.RootLogger)

	clusterConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	cLIConf.KubeConfigFilePath = *kubeconfigPath
	if err != nil {
		if _, ok := err.(*fs.PathError); !ok {
			return CLIConfig{}, err
		}
		clusterConfig, err = rest.InClusterConfig()
		if err != nil {
			return CLIConfig{}, err
		}
		cLIConf.KubeConfigFilePath = "" // reset as we use in cluster configuration
	}
	cLIConf.ClusterConfig = clusterConfig
	return cLIConf, nil
}