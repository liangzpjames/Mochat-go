package archivebridge

import (
	"context"
	"errors"
	"io"
)

// ProductionBindingSource exposes only tenant/corp routing metadata. It must
// not return decrypted provider credentials.
type ProductionBindingSource interface {
	ProductionBindings(context.Context) ([]Binding, error)
}

// DriverFactory resolves and decrypts controlled credentials at the startup
// boundary, then constructs the driver and the resource that owns its close.
type DriverFactory interface {
	NewFinanceDriver(context.Context, Binding) (FinanceDriver, io.Closer, error)
	NewDataZoneDriver(context.Context, Binding) (DataZoneDriver, io.Closer, error)
}

type registeredDriver struct {
	binding Binding
	closer  io.Closer
}

type DriverRegistrar struct {
	source     ProductionBindingSource
	factory    DriverFactory
	store      *Store
	registered []registeredDriver
	completed  bool
}

func NewDriverRegistrar(source ProductionBindingSource, factory DriverFactory, store *Store) (*DriverRegistrar, error) {
	if source == nil || factory == nil || store == nil {
		return nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	return &DriverRegistrar{source: source, factory: factory, store: store}, nil
}

func (r *DriverRegistrar) RegisterAll(ctx context.Context) (int, error) {
	if r == nil || ctx == nil {
		return 0, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	if r.completed {
		return len(r.registered), nil
	}
	bindings, err := r.source.ProductionBindings(ctx)
	if err != nil {
		return 0, preserveBridgeError(err, "ARCHIVE_BINDING_SOURCE_UNAVAILABLE")
	}
	if len(bindings) == 0 {
		return 0, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	seen := make(map[string]struct{}, len(bindings))
	for index := range bindings {
		if err := normalizeBinding(&bindings[index]); err != nil {
			return 0, err
		}
		key := bindingKey(bindings[index].TenantID, bindings[index].CorpID)
		if _, exists := seen[key]; exists {
			return 0, &BridgeError{Code: "ARCHIVE_BINDING_CONFLICT"}
		}
		seen[key] = struct{}{}
	}

	for _, binding := range bindings {
		closer, err := r.registerOne(ctx, binding)
		if err != nil {
			rollbackErr := r.closeRegistered()
			if rollbackErr != nil {
				return 0, errors.Join(err, rollbackErr)
			}
			return 0, err
		}
		r.registered = append(r.registered, registeredDriver{binding: binding, closer: closer})
	}
	r.completed = true
	return len(r.registered), nil
}

func (r *DriverRegistrar) registerOne(ctx context.Context, binding Binding) (io.Closer, error) {
	var (
		closer io.Closer
		err    error
	)
	switch binding.IntegrationMode {
	case ModeSelfBuilt:
		var driver FinanceDriver
		driver, closer, err = r.factory.NewFinanceDriver(ctx, binding)
		if err == nil && (driver == nil || closer == nil) {
			err = &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
		}
		if err == nil {
			err = r.store.RegisterFinance(binding, driver)
		}
	case ModeThirdPartyDelegated:
		var driver DataZoneDriver
		driver, closer, err = r.factory.NewDataZoneDriver(ctx, binding)
		if err == nil && (driver == nil || closer == nil) {
			err = &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
		}
		if err == nil {
			err = r.store.RegisterDataZone(binding, driver)
		}
	default:
		err = &BridgeError{Code: "ARCHIVE_BINDING_INVALID"}
	}
	if err == nil {
		return closer, nil
	}
	if closer != nil {
		_ = closer.Close()
	}
	return nil, preserveBridgeError(err, "ARCHIVE_DRIVER_INITIALIZATION_FAILED")
}

func (r *DriverRegistrar) Close() error {
	if r == nil {
		return nil
	}
	err := r.closeRegistered()
	r.completed = false
	return err
}

func (r *DriverRegistrar) closeRegistered() error {
	var closeErrors []error
	for index := len(r.registered) - 1; index >= 0; index-- {
		driver := r.registered[index]
		if err := r.store.Unregister(driver.binding); err != nil {
			closeErrors = append(closeErrors, &BridgeError{Code: "ARCHIVE_DRIVER_CLOSE_FAILED", Cause: err})
		}
		if err := driver.closer.Close(); err != nil {
			closeErrors = append(closeErrors, &BridgeError{Code: "ARCHIVE_DRIVER_CLOSE_FAILED", Cause: err})
		}
	}
	r.registered = nil
	return errors.Join(closeErrors...)
}

func preserveBridgeError(err error, fallbackCode string) error {
	if ErrorCode(err) != "" {
		return err
	}
	return &BridgeError{Code: fallbackCode, Cause: err}
}

// UnavailableProductionBindingSource is the fail-closed default until a
// protected production binding repository is wired into this process.
type UnavailableProductionBindingSource struct{}

func (UnavailableProductionBindingSource) ProductionBindings(context.Context) ([]Binding, error) {
	return nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
}

// UnavailableDriverFactory preserves the production factory boundary without
// substituting fixture drivers or accepting plaintext environment credentials.
type UnavailableDriverFactory struct{}

func (UnavailableDriverFactory) NewFinanceDriver(context.Context, Binding) (FinanceDriver, io.Closer, error) {
	return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
}

func (UnavailableDriverFactory) NewDataZoneDriver(context.Context, Binding) (DataZoneDriver, io.Closer, error) {
	return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
}
