package supply

import (
	"github.com/cloudfoundry/libbuildpack"
	"github.com/nnicora/spire-agent-sidecar-buildpack/src/utils"
	"html/template"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	spireServerAddressEnv         = "SPIRE_SERVER_ADDRESS"
	spireServerPortEnv            = "SPIRE_SERVER_PORT"
	spireTrustDomainEnv           = "SPIRE_TRUST_DOMAIN"
	spireEnvoyProxyEnv            = "SPIRE_ENVOY_PROXY"
	spireApplicationSpiffeIdEnv   = "SPIRE_APPLICATION_SPIFFE_ID"
	spireCloudFoundrySVIDStoreEnv = "SPIRE_CLOUDFOUNDRY_SVID_STORE"
)

type Command interface {
	Execute(string, io.Writer, io.Writer, string, ...string) error
	Output(string, string, ...string) (string, error)
	Run(cmd *exec.Cmd) error
}

type Manifest interface {
	DefaultVersion(depName string) (libbuildpack.Dependency, error)
	AllDependencyVersions(string) []string
	RootDir() string
}

type Installer interface {
	InstallDependency(dep libbuildpack.Dependency, outputDir string) error
	InstallOnlyVersion(string, string) error
}

type Stager interface {
	AddBinDependencyLink(string, string) error
	DepDir() string
	DepsIdx() string
	DepsDir() string
	BuildDir() string
	WriteProfileD(string, string) error
}

type Config struct {
	SpireAgent SpireAgentConfig `yaml:"spire-agent"`
	Dist       string           `yaml:"dist"`
}

type SpireAgentConfig struct {
	Version string `yaml:"version"`
}

type Supplier struct {
	Stager       Stager
	Manifest     Manifest
	Installer    Installer
	Log          *libbuildpack.Logger
	Config       Config
	Command      Command
	VersionLines map[string]string
}

func New(stager Stager, manifest Manifest, installer Installer, logger *libbuildpack.Logger, command Command) *Supplier {
	return &Supplier{
		Stager:    stager,
		Manifest:  manifest,
		Installer: installer,
		Log:       logger,
		Command:   command,
	}
}

func (s *Supplier) Run() error {
	s.Log.BeginStep("Supplying spire")

	if err := s.InstallCertificates(); err != nil {
		s.Log.Error("Failed to copy certificates; %s", err.Error())
		return err
	}

	if err := s.CopySpireAgentConf(); err != nil {
		s.Log.Error("Failed to configure spire-agent.conf file; %s", err.Error())
		return err
	}

	if err := s.InstallSpireAgent(); err != nil {
		s.Log.Error("Failed to copy spire-agent binary; %s", err.Error())
		return err
	}

	if err := s.InstallSpireAgentPlugins(); err != nil {
		s.Log.Error("Failed to copy plugins; %s", err.Error())
		return err
	}

	if err := s.CreateLaunchForSidecars(); err != nil {
		s.Log.Error("Failed to create the sidecar processes; %s", err.Error())
		return err
	}

	if err := s.Setup(); err != nil {
		s.Log.Error("Could not setup; %s", err.Error())
		return err
	}

	return nil
}

func (s *Supplier) InstallSpireAgent() error {
	if exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.DepDir(), "bin", "spire-agent")); err != nil {
		return err
	} else if exists {
		return nil
	}

	return libbuildpack.CopyFile(filepath.Join(s.Manifest.RootDir(), "binaries", "spire-agent"), filepath.Join(s.Stager.DepDir(), "bin", "spire-agent"))
}

func (s *Supplier) InstallCertificates() error {
	pluginsDir := filepath.Join(s.Manifest.RootDir(), "certificates")

	err := filepath.Walk(pluginsDir, func(srcPath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			s.Log.Error("Can't copy certificate: %s", err.Error())
			return err
		}
		dstPath := filepath.Join(s.Stager.DepDir(), "certificates", info.Name())
		if errCopy := libbuildpack.CopyFile(srcPath, dstPath); errCopy != nil {
			s.Log.Error("Can't copy file: %s; Source `%s`, destination `%s`", errCopy.Error(), srcPath, dstPath)
			return errCopy
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Supplier) InstallSpireAgentPlugins() error {
	pluginsDir := filepath.Join(s.Manifest.RootDir(), "binaries", "plugins")

	err := filepath.Walk(pluginsDir, func(srcPath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			s.Log.Error("Can't copy file: %s", err.Error())
			return err
		}
		dstPath := filepath.Join(s.Stager.DepDir(), "bin", info.Name())
		if errCopy := libbuildpack.CopyFile(srcPath, dstPath); errCopy != nil {
			s.Log.Error("Can't copy file: %s; Source `%s`, destination `%s`", errCopy.Error(), srcPath, dstPath)
			return errCopy
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Supplier) CreateLaunchForSidecars() error {
	launch := filepath.Join(s.Stager.DepDir(), "launch.yml")
	if _, err := libbuildpack.FileExists(launch); err != nil {
		return err
	}

	launchFile, err := os.Create(launch)
	if err != nil {
		return err
	}

	launchFile.WriteString("---\nprocesses:\n")

	spireAgentSidecarTmpl := filepath.Join(s.Manifest.RootDir(), "templates", "spire_agent-sidecar.tmpl")
	spireAgentSidecar := template.Must(template.ParseFiles(spireAgentSidecarTmpl))
	err = spireAgentSidecar.Execute(launchFile, map[string]interface{}{
		"Idx": s.Stager.DepsIdx(),
	})
	if err != nil {
		return err
	}

	envoyProxy := utils.VcapOrEnvWithDefault(spireEnvoyProxyEnv, "false")
	if strings.ToLower(envoyProxy) == "true" {
		envoyConfig := filepath.Join(s.Stager.DepDir(), "envoy-config.yaml")
		if _, err := libbuildpack.FileExists(launch); err != nil {
			return err
		}

		envoyConfigFile, err := os.Create(envoyConfig)
		if err != nil {
			return err
		}

		envoyProxyConfigTmpl := filepath.Join(s.Manifest.RootDir(), "templates", "custom-envoy-conf.tmpl")
		envoyProxyConfig := template.Must(template.ParseFiles(envoyProxyConfigTmpl))

		std, err := utils.VcapOrEnv(spireTrustDomainEnv)
		if err != nil {
			return err
		}
		sasid, err := utils.VcapOrEnv(spireApplicationSpiffeIdEnv)
		if err != nil {
			return err
		}

		err = envoyProxyConfig.Execute(envoyConfigFile, map[string]interface{}{
			"SpiffeID":    sasid,
			"TrustDomain": std,
		})
		if err != nil {
			return err
		}

		err = envoyConfigFile.Close()
		if err != nil {
			return err
		}

		envoyProxySidecarTmpl := filepath.Join(s.Manifest.RootDir(), "templates", "envoy_proxy-sidecar.tmpl")
		envoyProxySidecar := template.Must(template.ParseFiles(envoyProxySidecarTmpl))
		err = envoyProxySidecar.Execute(launchFile, map[string]interface{}{
			"Idx":    s.Stager.DepsIdx(),
			"BaseId": rand.Int63n(65000),
		})
		if err != nil {
			return err
		}
	}

	err = launchFile.Close()
	if err != nil {
		return err
	}

	return nil
}

func (s *Supplier) CopySpireAgentConf() error {
	conf := filepath.Join(s.Stager.DepDir(), "spire-agent.conf")
	if _, err := libbuildpack.FileExists(conf); err != nil {
		return err
	}

	f, err := os.Create(conf)
	if err != nil {
		return err
	}

	s.Log.Info("Spire agent conf: %s", conf)

	confTmpl := filepath.Join(s.Manifest.RootDir(), "templates", "spire-agent-conf.tmpl")
	t := template.Must(template.ParseFiles(confTmpl))

	ssa, err := utils.VcapOrEnv(spireServerAddressEnv)
	if err != nil {
		return err
	}
	ssp, err := utils.VcapOrEnv(spireServerPortEnv)
	if err != nil {
		return err
	}
	std, err := utils.VcapOrEnv(spireTrustDomainEnv)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Idx":                s.Stager.DepsIdx(),
		"SpireServerAddress": ssa,
		"SpireServerPort":    ssp,
		"TrustDomain":        std,
	}

	cfSvidStoreEnv := utils.VcapOrEnvWithDefault(spireCloudFoundrySVIDStoreEnv, "false")
	if strings.ToLower(cfSvidStoreEnv) == "true" {
		data["CloudFoundrySVIDStoreEnabled"] = true
	}
	err = t.Execute(f, data)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return nil
}

func (s *Supplier) Setup() error {
	configPath := filepath.Join(s.Stager.BuildDir(), "buildpack.yml")
	if exists, err := libbuildpack.FileExists(configPath); err != nil {
		return err
	} else if exists {
		if err := libbuildpack.NewYAML().Load(configPath, &s.Config); err != nil {
			return err
		}
	}

	var m struct {
		VersionLines map[string]string `yaml:"version_lines"`
	}
	if err := libbuildpack.NewYAML().Load(filepath.Join(s.Manifest.RootDir(), "manifest.yml"), &m); err != nil {
		return err
	}
	s.VersionLines = m.VersionLines

	// create logs directory in case if doesn't exist
	logsDirPath := filepath.Join(s.Stager.BuildDir(), "logs")
	if exists, err := libbuildpack.FileExists(logsDirPath); err != nil {
		return err
	} else if !exists {
		if err := os.MkdirAll(logsDirPath, os.ModePerm); err != nil {
			s.Log.Error("could not create 'logs' directory: %v", err.Error())
		}
	}

	return nil
}
