package api

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hxdcloud/gost-x/config"
	"github.com/hxdcloud/gost-x/config/parsing"
	"github.com/hxdcloud/gost-x/registry"
)

// swagger:parameters getServiceRequest
type getServiceRequest struct {
	// output format, one of yaml|json, default is json.
	// in: query
	Format string `form:"format" json:"format"`
}

// successful operation.
// swagger:response getServiceResponse
type getServiceResponse struct {
	Config *config.Config
}

func getService(ctx *gin.Context) {
	// swagger:route GET /config/services ConfigManagement getServiceRequest
	//
	// Get service list.
	//
	//     Security:
	//       basicAuth: []
	//
	//     Responses:
	//       200: createServiceResponse

	var req getConfigRequest
	ctx.ShouldBindQuery(&req)

	var resp getConfigResponse
	resp.Config = config.Global()

	buf := &bytes.Buffer{}
	switch req.Format {
	case "yaml":
	default:
		req.Format = "json"
	}

	resp.Config.WriteServices(buf, req.Format)

	contentType := "application/json"
	if req.Format == "yaml" {
		contentType = "text/x-yaml"
	}

	ctx.Data(http.StatusOK, contentType, buf.Bytes())
}

// swagger:parameters createServiceRequest
type createServiceRequest struct {
	// in: body
	Data config.ServiceConfig `json:"data"`
}

// successful operation.
// swagger:response createServiceResponse
type createServiceResponse struct {
	Data Response
}

func createService(ctx *gin.Context) {
	// swagger:route POST /config/services ConfigManagement createServiceRequest
	//
	// Create a new service, the name of the service must be unique in service list.
	//
	//     Security:
	//       basicAuth: []
	//
	//     Responses:
	//       200: createServiceResponse

	var req createServiceRequest
	ctx.ShouldBindJSON(&req.Data)

	if req.Data.Name == "" {
		writeError(ctx, ErrInvalid)
		return
	}

	if registry.ServiceRegistry().IsRegistered(req.Data.Name) {
		writeError(ctx, ErrDup)
		return
	}

	svc, err := parsing.ParseService(&req.Data)
	if err != nil {
		writeError(ctx, ErrCreate)
		return
	}

	if err := registry.ServiceRegistry().Register(req.Data.Name, svc); err != nil {
		svc.Close()
		writeError(ctx, ErrDup)
		return
	}

	go svc.Serve()

	cfg := config.Global()
	cfg.Services = append(cfg.Services, &req.Data)
	config.SetGlobal(cfg)

	ctx.JSON(http.StatusOK, Response{
		Msg: "OK",
	})
}

// swagger:parameters updateServiceRequest
type updateServiceRequest struct {
	// in: path
	// required: true
	Service string `uri:"service" json:"service"`
	// in: body
	Data config.ServiceConfig `json:"data"`
}

// successful operation.
// swagger:response updateServiceResponse
type updateServiceResponse struct {
	Data Response
}

func updateService(ctx *gin.Context) {
	// swagger:route PUT /config/services/{service} ConfigManagement updateServiceRequest
	//
	// Update service by name, the service must already exist.
	//
	//     Security:
	//       basicAuth: []
	//
	//     Responses:
	//       200: updateServiceResponse

	var req updateServiceRequest
	ctx.ShouldBindUri(&req)
	ctx.ShouldBindJSON(&req.Data)

	old := registry.ServiceRegistry().Get(req.Service)
	if old == nil {
		writeError(ctx, ErrNotFound)
		return
	}
	old.Close()

	req.Data.Name = req.Service

	svc, err := parsing.ParseService(&req.Data)
	if err != nil {
		writeError(ctx, ErrCreate)
		return
	}

	registry.ServiceRegistry().Unregister(req.Service)

	if err := registry.ServiceRegistry().Register(req.Service, svc); err != nil {
		svc.Close()
		writeError(ctx, ErrDup)
		return
	}

	go svc.Serve()

	cfg := config.Global()
	for i := range cfg.Services {
		if cfg.Services[i].Name == req.Service {
			cfg.Services[i] = &req.Data
			break
		}
	}
	config.SetGlobal(cfg)

	ctx.JSON(http.StatusOK, Response{
		Msg: "OK",
	})
}

// swagger:parameters deleteServiceRequest
type deleteServiceRequest struct {
	// in: path
	// required: true
	Service string `uri:"service" json:"service"`
}

// successful operation.
// swagger:response deleteServiceResponse
type deleteServiceResponse struct {
	Data Response
}

func deleteService(ctx *gin.Context) {
	// swagger:route DELETE /config/services/{service} ConfigManagement deleteServiceRequest
	//
	// Delete service by name.
	//
	//     Security:
	//       basicAuth: []
	//
	//     Responses:
	//       200: deleteServiceResponse

	var req deleteServiceRequest
	ctx.ShouldBindUri(&req)

	svc := registry.ServiceRegistry().Get(req.Service)
	if svc == nil {
		writeError(ctx, ErrNotFound)
		return
	}

	registry.ServiceRegistry().Unregister(req.Service)
	svc.Close()

	cfg := config.Global()
	services := cfg.Services
	cfg.Services = nil
	for _, s := range services {
		if s.Name == req.Service {
			continue
		}
		cfg.Services = append(cfg.Services, s)
	}
	config.SetGlobal(cfg)

	ctx.JSON(http.StatusOK, Response{
		Msg: "OK",
	})
}
