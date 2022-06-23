package supply

import (
	"fmt"
	"github.com/cloudfoundry/libbuildpack"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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

	if err := s.InstallSpireAgent(); err != nil {
		s.Log.Error("Failed to copy spire-agent: %s", err.Error())
		return err
	}

	if err := s.CopySpireAgentConf(); err != nil {
		s.Log.Error("Failed to copy agent.conf: %s", err.Error())
		return err
	}

	if err := s.Setup(); err != nil {
		s.Log.Error("Could not setup: %s", err.Error())
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

func (s *Supplier) CopySpireAgentConf() error {
	conf := filepath.Join(s.Stager.DepDir(), "bin", "agent.conf")
	if _, err := libbuildpack.FileExists(conf); err != nil {
		return err
	}

	err := libbuildpack.CopyFile(filepath.Join(s.Manifest.RootDir(), "configs", "agent.conf"), conf)
	if err != nil {
		return err
	}

	d := map[string]interface{}{
		"SpireServerAddress": os.Getenv("SPIRE_SERVER_ADDRESS"),
	}

	f, err := os.Create(conf)
	if err != nil {
		return err
	}

	s.Log.Info("Spire agent conf: %s", conf)

	confTmpl := filepath.Join(s.Manifest.RootDir(), "templates", "spire-agent-conf.tmpl")
	t := template.Must(template.ParseFiles(confTmpl))

	err = t.Execute(f, d)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(conf)
	if err != nil {
		return err
	}

	s.Log.Info("Spire conf [%s]", string(b))

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

	logsDirPath := filepath.Join(s.Stager.BuildDir(), "logs")
	if err := os.Mkdir(logsDirPath, os.ModePerm); err != nil {
		return fmt.Errorf("Could not create 'logs' directory: %v", err)
	}

	return nil
}
