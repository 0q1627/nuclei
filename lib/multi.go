package nuclei

import (
	"context"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/projectdiscovery/nuclei/v3/pkg/catalog/loader"
	"github.com/projectdiscovery/nuclei/v3/pkg/core"
	"github.com/projectdiscovery/nuclei/v3/pkg/core/inputs"
	"github.com/projectdiscovery/nuclei/v3/pkg/output"
	"github.com/projectdiscovery/nuclei/v3/pkg/parsers"
	"github.com/projectdiscovery/nuclei/v3/pkg/protocols"
	"github.com/projectdiscovery/nuclei/v3/pkg/protocols/common/contextargs"
	"github.com/projectdiscovery/nuclei/v3/pkg/types"
	"github.com/projectdiscovery/ratelimit"
	errorutil "github.com/projectdiscovery/utils/errors"
)

// unsafeOptions are those nuclei objects/instances/types
// that are required to run nuclei engine but are not thread safe
// hence they are ephemeral and are created on every ExecuteNucleiWithOpts invocation
// in ThreadSafeNucleiEngine
type unsafeOptions struct {
	executerOpts protocols.ExecutorOptions
	engine       *core.Engine
}

// createEphemeralObjects creates ephemeral nuclei objects/instances/types
func createEphemeralObjects(base *NucleiEngine, opts *types.Options) (*unsafeOptions, error) {
	u := &unsafeOptions{}
	u.executerOpts = protocols.ExecutorOptions{
		Output:          base.customWriter,
		Options:         opts,
		Progress:        base.customProgress,
		Catalog:         base.catalog,
		IssuesClient:    base.rc,
		RateLimiter:     base.rateLimiter,
		Interactsh:      base.interactshClient,
		HostErrorsCache: base.hostErrCache,
		Colorizer:       aurora.NewAurora(true),
		ResumeCfg:       types.NewResumeCfg(),
	}
	if opts.RateLimitMinute > 0 {
		u.executerOpts.RateLimiter = ratelimit.New(context.Background(), uint(opts.RateLimitMinute), time.Minute)
	} else if opts.RateLimit > 0 {
		u.executerOpts.RateLimiter = ratelimit.New(context.Background(), uint(opts.RateLimit), time.Second)
	} else {
		u.executerOpts.RateLimiter = ratelimit.NewUnlimited(context.Background())
	}
	u.engine = core.New(opts)
	u.engine.SetExecuterOptions(u.executerOpts)
	return u, nil
}

// ThreadSafeNucleiEngine is a tweaked version of nuclei.Engine whose methods are thread-safe
// and can be used concurrently. Non-thread-safe methods start with Global prefix
type ThreadSafeNucleiEngine struct {
	eng *NucleiEngine
}

// NewThreadSafeNucleiEngine creates a new nuclei engine with given options
// whose methods are thread-safe and can be used concurrently
// Note: Non-thread-safe methods start with Global prefix
func NewThreadSafeNucleiEngine(opts ...NucleiSDKOptions) (*ThreadSafeNucleiEngine, error) {
	// default options
	e := &NucleiEngine{
		opts: types.DefaultOptions(),
		mode: threadSafe,
	}
	for _, option := range opts {
		if err := option(e); err != nil {
			return nil, err
		}
	}
	if err := e.init(); err != nil {
		return nil, err
	}
	return &ThreadSafeNucleiEngine{eng: e}, nil
}

// GlobalLoadAllTemplates loads all templates from nuclei-templates repo
// This method will load all templates based on filters given at the time of nuclei engine creation in opts
func (e *ThreadSafeNucleiEngine) GlobalLoadAllTemplates() error {
	return e.eng.LoadAllTemplates()
}

// GlobalResultCallback sets a callback function which will be called for each result
func (e *ThreadSafeNucleiEngine) GlobalResultCallback(callback func(event *output.ResultEvent)) {
	e.eng.resultCallbacks = []func(*output.ResultEvent){callback}
}

// ExecuteWithCallback executes templates on targets and calls callback on each result(only if results are found)
// This method can be called concurrently and it will use some global resources but can be runned parallelly
// by invoking this method with different options and targets
// Note: Not all options are thread-safe. this method will throw error if you try to use non-thread-safe options
func (e *ThreadSafeNucleiEngine) ExecuteNucleiWithOpts(targets []string, opts ...NucleiSDKOptions) error {
	baseOpts := *e.eng.opts
	tmpEngine := &NucleiEngine{opts: &baseOpts, mode: threadSafe}
	for _, option := range opts {
		if err := option(tmpEngine); err != nil {
			return err
		}
	}
	// create ephemeral nuclei objects/instances/types using base nuclei engine
	unsafeOpts, err := createEphemeralObjects(e.eng, tmpEngine.opts)
	if err != nil {
		return err
	}

	// load templates
	workflowLoader, err := parsers.NewLoader(&unsafeOpts.executerOpts)
	if err != nil {
		return errorutil.New("Could not create workflow loader: %s\n", err)
	}
	unsafeOpts.executerOpts.WorkflowLoader = workflowLoader

	store, err := loader.New(loader.NewConfig(tmpEngine.opts, e.eng.catalog, unsafeOpts.executerOpts))
	if err != nil {
		return errorutil.New("Could not create loader client: %s\n", err)
	}
	store.Load()

	inputProvider := &inputs.SimpleInputProvider{
		Inputs: []*contextargs.MetaInput{},
	}

	// load targets
	for _, target := range targets {
		inputProvider.Set(target)
	}

	if len(store.Templates()) == 0 && len(store.Workflows()) == 0 {
		return ErrNoTemplatesAvailable
	}
	if inputProvider.Count() == 0 {
		return ErrNoTargetsAvailable
	}

	engine := core.New(tmpEngine.opts)
	engine.SetExecuterOptions(unsafeOpts.executerOpts)

	_ = engine.ExecuteScanWithOpts(store.Templates(), inputProvider, false)

	engine.WorkPool().Wait()
	return nil
}

// Close all resources used by nuclei engine
func (e *ThreadSafeNucleiEngine) Close() {
	e.eng.Close()
}
