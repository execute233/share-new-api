package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func registerProxyRoutes(apiRouter *gin.RouterGroup) {
	proxyRoute := apiRouter.Group("/proxy")
	proxyRoute.Use(middleware.AdminAuth())
	for _, route := range proxyPermissionRoutes {
		proxyRoute.Handle(route.method, route.path, middleware.RequirePermission(route.permission), route.handler)
	}
}

var proxyPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/", permission: authz.ProxyRead, handler: controller.ListProxies},
	{method: http.MethodGet, path: "/all", permission: authz.ProxyRead, handler: controller.ListActiveProxies},
	{method: http.MethodGet, path: "/:id", permission: authz.ProxyRead, handler: controller.GetProxy},
	{method: http.MethodPost, path: "/", permission: authz.ProxySensitiveWrite, handler: controller.CreateProxy},
	{method: http.MethodPost, path: "/quick-add", permission: authz.ProxySensitiveWrite, handler: controller.QuickAddProxies},
	{method: http.MethodPut, path: "/:id", permission: authz.ProxySensitiveWrite, handler: controller.UpdateProxy},
	{method: http.MethodDelete, path: "/:id", permission: authz.ProxySensitiveWrite, handler: controller.DeleteProxy},
	{method: http.MethodPost, path: "/:id/test", permission: authz.ProxyOperate, handler: controller.TestProxy},
	{method: http.MethodPost, path: "/:id/quality-check", permission: authz.ProxyOperate, handler: controller.CheckProxyQuality},
}
