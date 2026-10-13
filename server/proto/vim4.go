package proto

type GetEdidRsp struct {
	Edid string `json:"edid"`
}
type SwitchEdidReq struct {
	Edid string `form:"edid" validate:"required"`
}
type GetCustomEdidListRsp struct {
	EdidList []string `json:"edidList"`
}
type DeleteEdidReq struct {
	Edid string `form:"edid" validate:"required"`
}
type UploadEdidRsp struct {
	File string `json:"file"`
}
