package plugins

import (
	"fmt"
	"reflect"
	"sync"
)

type Plugin interface {
	Start() error
	Stop() error
}

type PluginManager struct {
	lock    sync.RWMutex
	plugins map[string]Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]Plugin),
	}
}

func (pm *PluginManager) Register(name string, plugin Plugin) error {
	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	if err := plugin.Start(); err != nil {
		return fmt.Errorf("failed to init plugin %q: %w", name, err)
	}

	pm.lock.Lock()
	defer pm.lock.Unlock()

	pm.plugins[name] = plugin
	return nil
}

func (pm *PluginManager) Unregister(name string) error {
	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}
	if err := plugin.Stop(); err != nil {
		return fmt.Errorf("failed to stop plugin %q: %w", name, err)
	}

	pm.lock.Lock()
	defer pm.lock.Unlock()
	delete(pm.plugins, name)
	return nil
}

func (pm *PluginManager) UnregisterAll() error {
	var err error
	for name := range pm.plugins {
		pluginErr := pm.Unregister(name)
		if pluginErr != nil {
			err = pluginErr
		}
	}
	return err
}

func (pm *PluginManager) Execute(pluginName string, methodName string, args ...interface{}) (interface{}, error) {
	pm.lock.RLock()
    plugin, exists := pm.plugins[pluginName]
    pm.lock.RUnlock()
    if !exists {
        return nil, fmt.Errorf("plugin %q not found", pluginName)
    }
    // Use reflection to get the plugin's method
    pluginValue := reflect.ValueOf(plugin)
    method := pluginValue.MethodByName(methodName)
    if !method.IsValid() {
        return nil, fmt.Errorf("method %q not found in plugin %q", methodName, pluginName)
    }

    // Validate and call the method
    inputArgs := make([]reflect.Value, len(args))
    for i, arg := range args {
        inputArgs[i] = reflect.ValueOf(arg)
    }

    // Call the method and handle results
    results := method.Call(inputArgs)
    if len(results) == 0 {
        return nil, nil
    }
    if len(results) == 1 {
        return results[0].Interface(), nil
    }

    // Return multiple results as a slice
    output := make([]interface{}, len(results))
    for i, result := range results {
        output[i] = result.Interface()
    }
    return output, nil
}