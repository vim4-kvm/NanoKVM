package router

import (
	"github.com/gin-gonic/gin"

	"NanoKVM-Server/authn"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/network"
)

func networkRouter(r *gin.Engine) {
	service := network.NewService()

	r.POST("/api/network/wifi", platformHandler("POST /api/network/wifi", service.ConnectWifiNoAuth))           // connect Wi-Fi without auth (only available in ap mode)
	r.POST("/api/network/wifi/verify", platformHandler("POST /api/network/wifi/verify", service.VerifyApLogin)) // verify ap login

	api := r.Group("/api").Use(middleware.CheckToken())

	api.POST("/network/wol", platformHandler("POST /network/wol", service.WakeOnLAN))    // wake on lan
	api.GET("/network/wol/mac", platformHandler("GET /network/wol/mac", service.GetMac)) // get mac list

	admin := r.Group("/api").Use(
		middleware.CheckToken(),
		middleware.RequireRole(authn.RoleAdmin),
	)
	admin.DELETE("/network/wol/mac", platformHandler("DELETE /network/wol/mac", service.DeleteMac))        // delete mac
	admin.POST("/network/wol/mac/name", platformHandler("POST /network/wol/mac/name", service.SetMacName)) // set mac name

	admin.GET("/network/wifi", platformHandler("GET /network/wifi", service.GetWifi))                                // get Wi-Fi information
	admin.POST("/network/wifi/connect", platformHandler("POST /network/wifi/connect", service.ConnectWifi))          // connect Wi-Fi
	admin.POST("/network/wifi/disconnect", platformHandler("POST /network/wifi/disconnect", service.DisconnectWifi)) // disconnect Wi-Fi
	admin.GET("/network/dns", platformHandler("GET /network/dns", service.GetDNS))                                   // get DNS configuration
	admin.POST("/network/dns", platformHandler("POST /network/dns", service.SetDNS))                                 // set DNS configuration
}
