package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/masterzen/winrm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WinrmCommand struct {
	providerRef infrav1.ScvmmProviderReference
	input       []byte
	output      chan WinrmResult
}

type WinrmResult struct {
	stdout []byte
	stderr []byte
	err    error
}

type WinrmErrorResult interface {
	GetError() string
}

// VMResult The result (passed as json) of a call to Scvmm scripts
type VMResult struct {
	Cloud          string
	Name           string
	Hostname       string
	Status         string
	Memory         int
	CpuCount       int
	VirtualNetwork string
	IPv4Addresses  []string
	VirtualDisks   []VMResultDisk
	ISOs           []struct {
		Size     int64
		Filename string
	}
	BiosGuid             string
	Id                   string
	VMId                 string
	AvailabilitySetNames []string          `json:"AvailabilitySetNames,omitempty"`
	Tag                  string            `json:"Tag,omitempty"`
	CustomProperty       map[string]string `json:"CustomProperty,omitempty"`
	Error                string            `json:"Error,omitempty"`
	ScriptErrors         string            `json:"ScriptErrors,omitempty"`
	Message              string            `json:"Message,omitempty"`
	CreationTime         metav1.Time       `json:"CreationTime,omitempty"`
	ModifiedTime         metav1.Time       `json:"ModifiedTime,omitempty"`
	Result               string            `json:"Result,omitempty"`
}

type VMResultDisk struct {
	Size        int64
	MaximumSize int64
	BUS         int64
	LUN         int64
	VMHost      string
	Path        string
	Filename    string
	IOPSMaximum int64 `json:"IOPSMaximum,omitempty"`
}

// GetError Implement GetError() for VMResult so it implements WinrmErrorResult
func (V VMResult) GetError() string {
	return V.Error
}

type VMSpecResult struct {
	infrav1.ScvmmMachineSpec
	Error        string
	ScriptErrors string
	Message      string
}

type VMIDsResult struct {
	Error string
	VMIDs []string
}

func (V VMIDsResult) GetError() string {
	return V.Error
}

type ScriptError struct {
	function string
	message  string
}

func (e *ScriptError) Error() string {
	return fmt.Sprintf("%s error: %s", e.function, e.message)
}

type WinrmProvider struct {
	Spec            infrav1.ScvmmProviderSpec
	ResourceVersion string
}

var (
	winrmCommandChannel chan WinrmCommand
	winrmTotal          = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "winrm",
			Subsystem: "calls",
			Name:      "total",
			Help:      "Number of winrm calls made",
		},
		[]string{"function"},
	)
	winrmErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "winrm",
			Subsystem: "calls",
			Name:      "errors_total",
			Help:      "Number of winrm calls made that returned an error",
		},
		[]string{"function"},
	)
	winrmDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "winrm",
			Subsystem: "calls",
			Name:      "duration_seconds",
			Help:      "Duration of winrm call in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"function"},
	)

	winrmProviders = make(map[infrav1.ScvmmProviderReference]WinrmProvider)
)

// Grab provider spec from the cache, needed for cloudinit iso generation
func getProvider(providerRef *infrav1.ScvmmProviderReference) (*infrav1.ScvmmProviderSpec, error) {
	if providerRef == nil {
		providerRef = &infrav1.ScvmmProviderReference{}
	}
	if provider, ok := winrmProviders[*providerRef]; ok {
		return &provider.Spec, nil
	}
	return nil, fmt.Errorf("ScvmmProvider %s/%s not found", providerRef.Namespace, providerRef.Name)
}

func CreateWinrmWorkers(numWorkers int) {
	metrics.Registry.MustRegister(winrmTotal, winrmErrors, winrmDuration)
	winrmCommandChannel = make(chan WinrmCommand)
	for i := 0; i < numWorkers; i++ {
		go winrmWorker(winrmCommandChannel, i+1)
	}
}

func StopWinrmWorkers() {
	close(winrmCommandChannel)
	// Wait a second for the worker goroutines to close
	time.Sleep(time.Second * 1)
}

func winrmWorker(inputs <-chan WinrmCommand, instance int) {
	log := ctrl.Log.WithName("winrmworker").WithValues("instance", instance)
	inp := WinrmCommand{}
	log.Info("Starting worker")
	for {
		// doWinrmWork could decide not to do work, which means it has to be redone in this next loop
		// This happens when the resourceVersion of the provider has changed,
		// or the command references a different provider
		if inp.input != nil {
			inp = doWinrmWork(inputs, inp, log)
		} else {
			var ok bool
			select {
			default:
				// Sleep here to give an already connected worker
				// the chance to pick up a command first
				time.Sleep(time.Millisecond * 100)
			case inp, ok = <-inputs:
				log.V(1).Info("got command", "inp", inp, "ok", ok)
				if !ok {
					log.Info("Finishing worker")
					return
				}
			}
		}
	}
}

// One connection, kept alive, to do commands.
// Will close after a timeout
func doWinrmWork(inputs <-chan WinrmCommand, inp WinrmCommand, log logr.Logger) WinrmCommand {
	providerRef := inp.providerRef
	log.V(1).Info("Starting connection", "providerref", providerRef)
	provider, ok := winrmProviders[providerRef]
	if !ok {
		err := fmt.Errorf("ScvmmProvider %s/%s not found",
			providerRef.Namespace, providerRef.Name)
		log.Error(err, "finding provider", "input", inp)
		winrmReturn(inp.output, nil, nil, err)
		return WinrmCommand{}
	}
	log.V(1).Info("Create WinrmCmd")
	cmd, err := createWinrmCmd(&provider.Spec, log)
	if err != nil {
		log.Error(err, "creating winrm cmd", "provider", provider)
		winrmReturn(inp.output, nil, nil, err)
		return WinrmCommand{}
	}
	defer func(cmd *winrm.DirectCommand) {
		err := cmd.Close()
		if err != nil {
			log.Error(err, "error when closing winrm cmd in defer")
		}
	}(cmd)
	for {
		if ExtraDebug {
			log.V(1).Info("Send Input", "input", string(inp.input))
		}
		if err := cmd.SendInput(inp.input, false); err != nil {
			log.Error(err, "winrm sendinput")
			winrmReturn(inp.output, nil, nil, err)
			return WinrmCommand{}
		}
		var stdout, stderr []byte
		for {
			var stderrline []byte
			stdout, stderrline, _, _, err = cmd.ReadOutput()
			if err != nil {
				log.Error(err, "winrm readoutput")
				winrmReturn(inp.output, nil, nil, err)
				return WinrmCommand{}
			}
			if ExtraDebug {
				log.V(1).Info("Got Output", "stdout", string(stdout), "stderr", string(stderrline))
			}
			// We want all stderr output
			stderr = append(stderr, stderrline...)
			if len(stdout) > 0 {
				// winrm returns line by line.
				// Our scripts are supposed to always return something
				// If nothing is coming, this should trigger a timeout
				break
			}
		}
		if ExtraDebug {
			log.V(1).Info("return output", "stdout", string(stdout), "stderr", string(stderr))
		}
		winrmReturn(inp.output, stdout, stderr, nil)
		if len(stderr) > 0 {
			// If there was something on stderr,
			// drop the connection to be on the safe side
			return WinrmCommand{}
		}
		keepalive := provider.Spec.KeepAliveSeconds
		if keepalive == 0 {
			keepalive = 20
		}
		var ok bool
		log.V(1).Info("getting new command", "keepalive", keepalive)
		select {
		case <-time.After(time.Second * time.Duration(keepalive)):
			// After keepalive seconds, close the connection by returning
			log.Info("keepalive timeout", "keepalive", keepalive)
			return WinrmCommand{}
		case inp, ok = <-inputs:
			log.V(1).Info("got new command", "inp", inp, "ok", ok)
			if !ok || providerRef != inp.providerRef ||
				winrmProviders[providerRef].ResourceVersion != provider.ResourceVersion {
				// Drop out of this function to reload the provider
				// Pass back this input for reprocessing
				log.Info("new input provider mismatch",
					"ok", ok,
					"providerNew", inp.providerRef,
					"providerOld", providerRef,
					"versionNew", winrmProviders[providerRef].ResourceVersion,
					"versionOld", provider.ResourceVersion,
				)
				return inp
			}
			log.V(1).Info("winrm kept alive")
			// Otherwise, continue the loop, keep using the same cmd connection
		}
	}
}

func winrmReturn(ret chan WinrmResult, stdout []byte, stderr []byte, err error) {
	ret <- WinrmResult{
		stdout: stdout,
		stderr: stderr,
		err:    err,
	}
	close(ret)
}

func getFuncScript(provider *infrav1.ScvmmProviderSpec) ([]byte, error) {
	funcScripts := make(map[string][]byte)
	scriptfiles, err := filepath.Glob(os.Getenv("SCRIPT_DIR") + "/*.ps1")
	if err != nil {
		return nil, fmt.Errorf("error scanning script dir %s: %v", os.Getenv("SCRIPT_DIR"), err)
	}
	for _, file := range scriptfiles {
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading script file %s: %v", file, err)
		}
		funcScripts[name] = content
	}
	for name, content := range provider.ExtraFunctions {
		funcScripts[name] = []byte(content)
	}
	var functionScripts bytes.Buffer
	functionScripts.WriteString("$ProgressPreference = 'SilentlyContinue'\n" +
		"$WarningPreference = 'SilentlyContinue'\n" +
		"$VerbosePreference = 'SilentlyContinue'\n" +
		"$InformationPreference = 'SilentlyContinue'\n" +
		"$DebugPreference = 'SilentlyContinue'\n\n")
	for name, value := range provider.Env {
		functionScripts.WriteString("${env:" + name + "} = '" + escapeSingleQuotes(value) + "'\n")
	}
	for name, value := range provider.SensitiveEnv {
		functionScripts.WriteString("${env:" + name + "} = '" + escapeSingleQuotes(value) + "'\n")
	}
	for name, content := range funcScripts {
		functionScripts.WriteString("\nfunction " + name + " {\n")
		functionScripts.Write(content)
		functionScripts.WriteString("}\n\n")
	}
	return functionScripts.Bytes(), nil
}

func createWinrmCmd(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.DirectCommand, error) {
	functionScript, err := getFuncScript(provider)
	if err != nil {
		return &winrm.DirectCommand{}, err
	}
	cmd, err := createWinrmPowershell(provider, log)
	if err != nil {
		return &winrm.DirectCommand{}, err
	}
	if err := sendWinrmFunctions(log, cmd, functionScript); err != nil {
		closeErr := cmd.Close()
		if closeErr != nil {
			log.Error(closeErr, "error when closing winrm cmd after sendWinrmFunctions error")
		}
		return &winrm.DirectCommand{}, err
	}
	if err := sendWinrmConnect(log, cmd, provider.ScvmmHost); err != nil {
		closeErr := cmd.Close()
		if closeErr != nil {
			log.Error(closeErr, "error when closing winrm cmd after sendWinrmConnect error")
		}
		return &winrm.DirectCommand{}, err
	}
	return cmd, nil
}

func createWinrmConnection(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.Client, error) {
	defer winrmTimer("CreateConnection")()
	endpoint := winrm.NewEndpoint(provider.ExecHost, 5985, false, false, nil, nil, nil, 60)
	// Don't use winrm.DefaultParameters here because of concurrency issues
	params := winrm.NewParameters("PT60S", "en-US", 153600)
	params.RequestOptions["WINRS_NOPROFILE"] = "TRUE"
	params.RequestOptions["WINRS_CONSOLEMODE_STDIN"] = "FALSE"
	params.RequestOptions["WINRS_SKIP_CMD_SHELL"] = "TRUE"
	// params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	enc, err := winrm.NewEncryption("ntlm")
	if err != nil {
		winrmErrors.WithLabelValues("CreateConnection").Inc()
		return nil, errors.Wrap(err, "Creating winrm client")
	}
	params.TransportDecorator = func() winrm.Transporter { return enc }

	if ExtraDebug {
		log.V(1).Info("Creating WinRM connection", "host", provider.ExecHost, "port", 5985)
	}
	winrmClient, err := winrm.NewClientWithParameters(endpoint, provider.ScvmmUsername, provider.ScvmmPassword, params)
	if err != nil {
		winrmErrors.WithLabelValues("CreateConnection").Inc()
		return nil, errors.Wrap(err, "Creating winrm client")
	}
	return winrmClient, nil
}

func createWinrmShell(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.Shell, error) {
	winrmClient, err := createWinrmConnection(provider, log)
	if err != nil {
		return nil, err
	}
	defer winrmTimer("CreateShell")()
	if ExtraDebug {
		log.V(1).Info("Creating WinRM shell")
	}
	shell, err := winrmClient.CreateShell()
	if err != nil {
		winrmErrors.WithLabelValues("CreateShell").Inc()
		return nil, errors.Wrap(err, "Creating winrm shell")
	}
	return shell, nil
}

func createWinrmPowershell(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.DirectCommand, error) {
	shell, err := createWinrmShell(provider, log)
	if err != nil {
		return nil, err
	}
	if ExtraDebug {
		log.V(1).Info("Starting WinRM powershell.exe")
	}
	defer winrmTimer("powershell.exe")()
	cmd, err := shell.ExecuteDirect("powershell.exe", "-NonInteractive", "-NoProfile", "-Command", "-")
	if err != nil {
		winrmErrors.WithLabelValues("powershell.exe").Inc()
		return nil, errors.Wrap(err, "Creating winrm powershell")
	}
	if err := sendWinrmPing(log, cmd, "Creating winrm powershell"); err != nil {
		winrmErrors.WithLabelValues("powershell.exe").Inc()
		return nil, err
	}
	return cmd, nil
}

func sendWinrmPing(log logr.Logger, cmd *winrm.DirectCommand, what string) error {
	log.V(1).Info("Sending WinRM ping")
	if err := cmd.SendCommand("Write-Host 'OK'"); err != nil {
		return errors.Wrap(err, what+", Pinging")
	}
	log.V(1).Info("Getting WinRM ping")
	stdout, stderr, _, _, err := cmd.ReadOutput()
	if err != nil {
		return errors.Wrap(err, what+", Reading result")
	}
	log.V(1).Info("Got WinRM ping", "stdout", string(stdout), "stderr", string(stderr))
	if strings.TrimSpace(string(stdout)) != "OK" {
		return errors.New(what + " result: " + string(stdout) + " (ERR=" + string(stderr))
	}
	return nil
}

func sendWinrmFunctions(log logr.Logger, cmd *winrm.DirectCommand, functionScript []byte) error {
	if ExtraDebug {
		log.V(1).Info("Sending WinRM function script")
	}
	defer winrmTimer("SendFunctions")()
	if err := cmd.SendInput(functionScript, false); err != nil {
		winrmErrors.WithLabelValues("SendFunctions").Inc()
		return errors.Wrap(err, "Sending powershell functions")
	}
	if err := sendWinrmPing(log, cmd, "Sending powerwhell functions"); err != nil {
		winrmErrors.WithLabelValues("SendFunctions").Inc()
		return err
	}
	return nil
}

func sendWinrmConnect(log logr.Logger, cmd *winrm.DirectCommand, scvmmHost string) error {
	defer winrmTimer("ConnectSCVMM")()
	if ExtraDebug {
		log.V(1).Info("Calling WinRM function ConnectSCVMM")
	}
	if err := cmd.SendCommand("ConnectSCVMM -Computername '%s'", scvmmHost); err != nil {
		winrmErrors.WithLabelValues("ConnectSCVMM").Inc()
		return errors.Wrap(err, "Connecting to SCVMM")
	}
	if err := sendWinrmPing(log, cmd, "Connecting to SCVMM"); err != nil {
		winrmErrors.WithLabelValues("ConnectSCVMM").Inc()
		return err
	}
	return nil
}

func sendWinrmCommandWithErrorResult[T WinrmErrorResult](log logr.Logger, providerRef *infrav1.ScvmmProviderReference, command string, args ...interface{}) (T, error) {
	var res T
	if ExtraDebug {
		log.V(1).Info("Sending WinRM command", "command", command, "args", args,
			"cmdline", fmt.Sprintf(command+"\n", args...))
	}
	funcName, _, _ := strings.Cut(command, " ")
	log.V(1).Info("Call " + funcName)
	defer winrmTimer(funcName)()
	output := make(chan WinrmResult)
	if providerRef == nil {
		providerRef = &infrav1.ScvmmProviderReference{}
	}
	winrmCommandChannel <- WinrmCommand{
		providerRef: *providerRef,
		input:       []byte((fmt.Sprintf(command+"\n", args...))),
		output:      output,
	}
	log.V(1).Info("waiting for output", "funcname", funcName)
	result, ok := <-output
	if !ok {
		winrmErrors.WithLabelValues(funcName).Inc()
		return res, errors.Wrap(result.err, "Failed to get function output "+funcName)
	}
	if result.err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return res, errors.Wrap(result.err, "Failed to call function "+funcName)
	}
	if ExtraDebug {
		log.V(1).Info("Got WinRM Result", "stdout", string(result.stdout), "stderr", string(result.stderr))
	}
	if err := json.Unmarshal(result.stdout, &res); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return res, errors.Wrap(err, "Decode result error: "+string(result.stdout)+"  (stderr="+string(result.stderr)+")")
	}
	if res.GetError() != "" {
		err := &ScriptError{function: funcName, message: res.GetError()}
		log.V(1).Error(err, "Script error", "function", funcName, "stacktrace", res.GetError())
		winrmErrors.WithLabelValues(funcName).Inc()
		return res, err
	}
	log.V(1).Info(funcName+" Result", "vm", res)
	return res, nil
}

func sendWinrmCommand(log logr.Logger, providerRef *infrav1.ScvmmProviderReference, command string, args ...interface{}) (VMResult, error) {
	return sendWinrmCommandWithErrorResult[VMResult](log, providerRef, command, args...)
}

func winrmTimer(funcName string) func() {
	winrmTotal.WithLabelValues(funcName).Inc()
	start := time.Now()
	return func() {
		winrmDuration.WithLabelValues(funcName).Observe(time.Since(start).Seconds())
	}
}

func sendWinrmSpecCommand(log logr.Logger, providerRef *infrav1.ScvmmProviderReference, command string, scvmmMachine *infrav1.ScvmmMachine) (VMSpecResult, error) {
	specjson, err := json.Marshal(scvmmMachine.Spec)
	if err != nil {
		return VMSpecResult{}, errors.Wrap(err, "encoding spec")
	}
	metajson, err := json.Marshal(scvmmMachine.ObjectMeta)
	if err != nil {
		return VMSpecResult{}, errors.Wrap(err, "encoding metadata")
	}
	cmdline := fmt.Sprintf(command+" -spec '%s' -metadata '%s'\n",
		escapeSingleQuotes(string(specjson)),
		escapeSingleQuotes(string(metajson)))
	if ExtraDebug {
		log.V(1).Info("Sending WinRM command", "command", command, "spec", scvmmMachine.Spec,
			"metadata", scvmmMachine.ObjectMeta,
			"cmdline", cmdline)
	}
	funcName, _, _ := strings.Cut(command, " ")
	defer winrmTimer(funcName)()
	output := make(chan WinrmResult)
	if providerRef == nil {
		providerRef = &infrav1.ScvmmProviderReference{}
	}
	winrmCommandChannel <- WinrmCommand{
		providerRef: *providerRef,
		input:       []byte(cmdline),
		output:      output,
	}
	result := <-output
	if result.err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, errors.Wrap(result.err, "Failed to call function "+funcName)
	}
	if ExtraDebug {
		log.V(1).Info("Got WinRMSpec Result", "stdout", string(result.stdout), "stderr", string(result.stderr))
	}
	var res VMSpecResult
	if err := json.Unmarshal(result.stdout, &res); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, errors.Wrap(err, "Decode result error: "+string(result.stdout)+
			"  (stderr="+string(result.stderr)+")")
	}
	if res.Error != "" {
		err := &ScriptError{function: funcName, message: res.Message}
		log.V(1).Error(err, "Script error", "function", funcName, "stacktrace", res.Error)
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, err
	}
	return res, nil
}
