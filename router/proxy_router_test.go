package router

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/service/authz"
)

func TestProxyRoutesUseDedicatedPermissions(t *testing.T) {
	tests := []struct {
		method     string
		path       string
		permission authz.Permission
		handler    any
	}{
		{http.MethodGet, "/", authz.ProxyRead, controller.ListProxies},
		{http.MethodGet, "/all", authz.ProxyRead, controller.ListActiveProxies},
		{http.MethodPost, "/:id/test", authz.ProxyOperate, controller.TestProxy},
		{http.MethodPost, "/:id/quality-check", authz.ProxyOperate, controller.CheckProxyQuality},
		{http.MethodPost, "/", authz.ProxySensitiveWrite, controller.CreateProxy},
		{http.MethodPut, "/:id", authz.ProxySensitiveWrite, controller.UpdateProxy},
		{http.MethodDelete, "/:id", authz.ProxySensitiveWrite, controller.DeleteProxy},
	}
	for _, test := range tests {
		matched := false
		for _, route := range proxyPermissionRoutes {
			if route.method == test.method && route.path == test.path {
				matched = route.permission == test.permission
				break
			}
		}
		if !matched {
			t.Fatalf("missing permission route %s %s", test.method, test.path)
		}
	}
}
