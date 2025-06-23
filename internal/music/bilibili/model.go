package bilibili

import (
	"io"
)

const (
	WebInterfaceView = "https://api.bilibili.com/x/web-interface/view"
	PlayerPlayUrl    = "https://api.bilibili.com/x/player/playurl"
)

var (
	SessionData = ""
)

type Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ttl     int    `json:"ttl"`
}

type Response[RespData WebInterfaceViewRespData | VideoPlayRespData] struct {
	Resp
	Data              RespData   `json:"data"`
	IsSeasonDisplay   bool       `json:"is_season_display"`
	UserGarb          UserGarb   `json:"user_garb"`
	HonorReply        HonorReply `json:"honor_reply"`
	LikeIcon          string     `json:"like_icon"`
	NeedJumpBv        bool       `json:"need_jump_bv"`
	DisableShowUpInfo bool       `json:"disable_show_up_info"`
}

type WebInterfaceViewRespData struct {
	Bvid               string      `json:"bvid"`
	Aid                int         `json:"aid"`
	Videos             int         `json:"videos"`
	Tid                int         `json:"tid"`
	Tname              string      `json:"tname"`
	Copyright          int         `json:"copyright"`
	Pic                string      `json:"pic"`
	Title              string      `json:"title"`
	Pubdate            int         `json:"pubdate"`
	Ctime              int         `json:"ctime"`
	Desc               string      `json:"desc"`
	DescV2             []DescV2    `json:"desc_v2"`
	State              int         `json:"state"`
	Duration           int         `json:"duration"`
	Rights             Rights      `json:"rights"`
	Owner              Owner       `json:"owner"`
	Stat               Stat        `json:"stat"`
	Dynamic            string      `json:"dynamic"`
	Cid                int         `json:"cid"`
	Dimension          Dimension   `json:"dimension"`
	SeasonId           int         `json:"season_id"`
	Premiere           interface{} `json:"premiere"`
	TeenageMode        int         `json:"teenage_mode"`
	IsChargeableSeason bool        `json:"is_chargeable_season"`
	IsStory            bool        `json:"is_story"`
	IsUpowerExclusive  bool        `json:"is_upower_exclusive"`
	IsUpowerPlay       bool        `json:"is_upower_play"`
	EnableVt           int         `json:"enable_vt"`
	VtDisplay          string      `json:"vt_display"`
	NoCache            bool        `json:"no_cache"`
	Pages              []PageInfo  `json:"pages"`
	Subtitle           Subtitle    `json:"subtitle"`
	UgcSeason          UgcSeason   `json:"ugc_season"`
	EpCount            int         `json:"ep_count"`
	SeasonType         int         `json:"season_type"`
	IsPaySeason        bool        `json:"is_pay_season"`
}

type Episodes struct {
	SeasonId  int      `json:"season_id"`
	SectionId int      `json:"section_id"`
	Id        int      `json:"id"`
	Aid       int      `json:"aid"`
	Cid       int      `json:"cid"`
	Title     string   `json:"title"`
	Attribute int      `json:"attribute"`
	Arc       Arc      `json:"arc"`
	Page      PageInfo `json:"page"`
	Bvid      string   `json:"bvid"`
}
type Sections struct {
	SeasonId int        `json:"season_id"`
	Id       int        `json:"id"`
	Title    string     `json:"title"`
	Type     int        `json:"type"`
	Episodes []Episodes `json:"episodes"`
}
type UgcSeason struct {
	Id        int        `json:"id"`
	Title     string     `json:"title"`
	Cover     string     `json:"cover"`
	Mid       int        `json:"mid"`
	Intro     string     `json:"intro"`
	SignState int        `json:"sign_state"`
	Attribute int        `json:"attribute"`
	Sections  []Sections `json:"sections"`
}

type Arc struct {
	Aid                int         `json:"aid"`
	Videos             int         `json:"videos"`
	TypeId             int         `json:"type_id"`
	TypeName           string      `json:"type_name"`
	Copyright          int         `json:"copyright"`
	Pic                string      `json:"pic"`
	Title              string      `json:"title"`
	PubDate            int         `json:"pubdate"`
	Ctime              int         `json:"ctime"`
	Desc               string      `json:"desc"`
	State              int         `json:"state"`
	Duration           int         `json:"duration"`
	Rights             Rights      `json:"rights"`
	Author             Author      `json:"author"`
	Stat               Stat        `json:"stat"`
	Dynamic            string      `json:"dynamic"`
	Dimension          Dimension   `json:"dimension"`
	DescV2             interface{} `json:"desc_v2"`
	IsChargeableSeason bool        `json:"is_chargeable_season"`
	IsBlooper          bool        `json:"is_blooper"`
	EnableVt           int         `json:"enable_vt"`
	VtDisplay          string      `json:"vt_display"`
}

type SubtitleAuthor struct {
	Mid            int64  `json:"mid"`
	Name           string `json:"name"`
	Sex            string `json:"sex"`
	Face           string `json:"face"`
	Sign           string `json:"sign"`
	Rank           int    `json:"rank"`
	Birthday       int64  `json:"birthday"`
	IsFakeAccount  int    `json:"is_fake_account"`
	IsDeleted      int    `json:"is_deleted"`
	InRegAudit     int    `json:"in_reg_audit"`
	IsSeniorMember int    `json:"is_senior_member"`
}
type SubtitleDetail struct {
	ID          int64          `json:"id"`
	Lan         string         `json:"lan"`
	LanDoc      string         `json:"lan_doc"`
	IsLock      bool           `json:"is_lock"`
	SubtitleURL string         `json:"subtitle_url"`
	Type        int            `json:"type"`
	IDStr       string         `json:"id_str"`
	AIType      int            `json:"ai_type"`
	AIStatus    int            `json:"ai_status"`
	Author      SubtitleAuthor `json:"author"`
}
type Subtitle struct {
	AllowSubmit bool             `json:"allow_submit"`
	List        []SubtitleDetail `json:"list"`
}
type PageInfo struct {
	Cid       int       `json:"cid"`
	Page      int       `json:"page"`
	From      string    `json:"from"`
	Part      string    `json:"part"`
	Duration  int       `json:"duration"`
	Vid       string    `json:"vid"`
	Weblink   string    `json:"weblink"`
	Dimension Dimension `json:"dimension"`
}

type Author struct {
	Mid  int    `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}
type Dimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Rotate int `json:"rotate"`
}
type Stat struct {
	Aid        int    `json:"aid"`
	View       int    `json:"view"`
	Danmaku    int    `json:"danmaku"`
	Reply      int    `json:"reply"`
	Fav        int    `json:"fav"`
	Favorite   int    `json:"favorite"`
	Coin       int    `json:"coin"`
	Share      int    `json:"share"`
	NowRank    int    `json:"now_rank"`
	HisRank    int    `json:"his_rank"`
	Like       int    `json:"like"`
	Dislike    int    `json:"dislike"`
	Evaluation string `json:"evaluation"`
	ArgueMsg   string `json:"argue_msg"`
	Vt         int    `json:"vt"`
	Vv         int    `json:"vv"`
}
type Owner struct {
	Mid  int    `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}
type Rights struct {
	Bp            int `json:"bp"`
	Elec          int `json:"elec"`
	Download      int `json:"download"`
	Movie         int `json:"movie"`
	Pay           int `json:"pay"`
	Hd5           int `json:"hd5"`
	NoReprint     int `json:"no_reprint"`
	Autoplay      int `json:"autoplay"`
	UgcPay        int `json:"ugc_pay"`
	IsCooperation int `json:"is_cooperation"`
	UgcPayPreview int `json:"ugc_pay_preview"`
	NoBackground  int `json:"no_background"`
	CleanMode     int `json:"clean_mode"`
	IsSteinGate   int `json:"is_stein_gate"`
	Is360         int `json:"is_360"`
	NoShare       int `json:"no_share"`
	ArcPay        int `json:"arc_pay"`
	FreeWatch     int `json:"free_watch"`
}
type DescV2 struct {
	RawText string `json:"raw_text"`
	Type    int    `json:"type"`
	BizId   int    `json:"biz_id"`
}

type UserGarb struct {
	UrlImageAniCut string `json:"url_image_ani_cut"`
}

type HonorReply struct {
	Honor []Honor `json:"honor"`
}

type Honor struct {
	Aid                int    `json:"aid"`
	Type               int    `json:"type"`
	Desc               string `json:"desc"`
	WeeklyRecommendNum int    `json:"weekly_recommend_num"`
}

type VideoPlayRespData struct {
	From              string        `json:"from"`
	Result            string        `json:"result"`
	Message           string        `json:"message"`
	Quality           int           `json:"quality"`
	Format            string        `json:"format"`
	Timelength        int           `json:"timelength"`
	AcceptFormat      string        `json:"accept_format"`
	AcceptDescription []string      `json:"accept_description"`
	AcceptQuality     []int         `json:"accept_quality"`
	VideoCodecid      int           `json:"video_codecid"`
	SeekParam         string        `json:"seek_param"`
	SeekType          string        `json:"seek_type"`
	Dash              VideoDash     `json:"dash"`
	SupportFormats    []VideoFormat `json:"support_formats"`
	HighFormat        interface{}   `json:"high_format"`
	LastPlayTime      int           `json:"last_play_time"`
	LastPlayCid       int           `json:"last_play_cid"`
}

type VideoDash struct {
	Duration         int           `json:"duration"`
	MinBufferTime    float64       `json:"minBufferTime"`
	MinBufferTimeAlt float64       `json:"min_buffer_time"`
	Video            []VideoStream `json:"video"`
	Audio            []AudioStream `json:"audio"`
	Dolby            struct {
		Type  int         `json:"type"`
		Audio interface{} `json:"audio"`
	} `json:"dolby"`
	Flac interface{} `json:"flac"`
}

type VideoStream struct {
	ID              int      `json:"id"`
	BaseURL         string   `json:"baseUrl"`
	BaseURLAlt      string   `json:"base_url"`
	BackupURL       []string `json:"backupUrl"`
	BackupURLAlt    []string `json:"backup_url"`
	Bandwidth       int      `json:"bandwidth"`
	MimeType        string   `json:"mimeType"`
	MimeTypeAlt     string   `json:"mime_type"`
	Codecs          string   `json:"codecs"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	FrameRate       string   `json:"frameRate"`
	FrameRateAlt    string   `json:"frame_rate"`
	Sar             string   `json:"sar"`
	StartWithSap    int      `json:"startWithSap"`
	StartWithSapAlt int      `json:"start_with_sap"`
	SegmentBase     struct {
		Initialization string `json:"Initialization"`
		IndexRange     string `json:"indexRange"`
	} `json:"SegmentBase"`
	SegmentBaseAlt struct {
		Initialization string `json:"initialization"`
		IndexRange     string `json:"index_range"`
	} `json:"segment_base"`
	Codecid int `json:"codecid"`
}

type AudioStream struct {
	ID              int      `json:"id"`
	BaseURL         string   `json:"baseUrl"`
	BaseURLAlt      string   `json:"base_url"`
	BackupURL       []string `json:"backupUrl"`
	BackupURLAlt    []string `json:"backup_url"`
	Bandwidth       int      `json:"bandwidth"`
	MimeType        string   `json:"mimeType"`
	MimeTypeAlt     string   `json:"mime_type"`
	Codecs          string   `json:"codecs"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	FrameRate       string   `json:"frameRate"`
	FrameRateAlt    string   `json:"frame_rate"`
	Sar             string   `json:"sar"`
	StartWithSap    int      `json:"startWithSap"`
	StartWithSapAlt int      `json:"start_with_sap"`
	SegmentBase     struct {
		Initialization string `json:"Initialization"`
		IndexRange     string `json:"indexRange"`
	} `json:"SegmentBase"`
	SegmentBaseAlt struct {
		Initialization string `json:"initialization"`
		IndexRange     string `json:"index_range"`
	} `json:"segment_base"`
	Codecid int `json:"codecid"`
}

type VideoFormat struct {
	Quality        int      `json:"quality"`
	Format         string   `json:"format"`
	NewDescription string   `json:"new_description"`
	DisplayDesc    string   `json:"display_desc"`
	Superscript    string   `json:"superscript"`
	Codecs         []string `json:"codecs"`
}

type NavUserRespData struct {
	IsLogin            bool           `json:"isLogin"`
	EmailVerified      int            `json:"email_verified"`
	Face               string         `json:"face"`
	FaceNft            int            `json:"face_nft"`
	FaceNftType        int            `json:"face_nft_type"`
	LevelInfo          LevelInfo      `json:"level_info"`
	Mid                int            `json:"mid"`
	MobileVerified     int            `json:"mobile_verified"`
	Money              float64        `json:"money"`
	Moral              int            `json:"moral"`
	Official           Official       `json:"official"`
	OfficialVerify     OfficialVerify `json:"officialVerify"`
	Pendant            Pendant        `json:"pendant"`
	Scores             int            `json:"scores"`
	Uname              string         `json:"uname"`
	VipDueDate         int64          `json:"vipDueDate"`
	VipStatus          int            `json:"vipStatus"`
	VipType            int            `json:"vipType"`
	VipPayType         int            `json:"vip_pay_type"`
	VipThemeType       int            `json:"vip_theme_type"`
	VipLabel           VipLabel       `json:"vip_label"`
	VipAvatarSubscript int            `json:"vip_avatar_subscript"`
	VipNicknameColor   string         `json:"vip_nickname_color"`
	Vip                Vip            `json:"vip"`
	Wallet             Wallet         `json:"wallet"`
	HasShop            bool           `json:"has_shop"`
	ShopUrl            string         `json:"shop_url"`
	AllowanceCount     int            `json:"allowance_count"`
	AnswerStatus       int            `json:"answer_status"`
	IsSeniorMember     int            `json:"is_senior_member"`
	WbiImg             WbiImg         `json:"wbi_img"`
	IsJury             bool           `json:"is_jury"`
}

type LevelInfo struct {
	CurrentLevel int `json:"current_level"`
	CurrentMin   int `json:"current_min"`
	CurrentExp   int `json:"current_exp"`
	NextExp      int `json:"next_exp"`
}

type Official struct {
	Role  int    `json:"role"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Type  int    `json:"type"`
}

type OfficialVerify struct {
	Type int    `json:"type"`
	Desc string `json:"desc"`
}

type Pendant struct {
	Pid               int    `json:"pid"`
	Name              string `json:"name"`
	Image             string `json:"image"`
	Expire            int    `json:"expire"`
	ImageEnhance      string `json:"image_enhance"`
	ImageEnhanceFrame string `json:"image_enhance_frame"`
}

type VipLabel struct {
	Path                  string `json:"path"`
	Text                  string `json:"text"`
	LabelTheme            string `json:"label_theme"`
	TextColor             string `json:"text_color"`
	BgStyle               int    `json:"bg_style"`
	BgColor               string `json:"bg_color"`
	BorderColor           string `json:"border_color"`
	UseImgLabel           bool   `json:"use_img_label"`
	ImgLabelUriHans       string `json:"img_label_uri_hans"`
	ImgLabelUriHant       string `json:"img_label_uri_hant"`
	ImgLabelUriHansStatic string `json:"img_label_uri_hans_static"`
	ImgLabelUriHantStatic string `json:"img_label_uri_hant_static"`
}

type Vip struct {
	Type               int      `json:"type"`
	Status             int      `json:"status"`
	DueDate            int64    `json:"due_date"`
	VipPayType         int      `json:"vip_pay_type"`
	ThemeType          int      `json:"theme_type"`
	Label              VipLabel `json:"label"`
	AvatarSubscript    int      `json:"avatar_subscript"`
	NicknameColor      string   `json:"nickname_color"`
	Role               int      `json:"role"`
	AvatarSubscriptUrl string   `json:"avatar_subscript_url"`
	TvVipStatus        int      `json:"tv_vip_status"`
	TvVipPayType       int      `json:"tv_vip_pay_type"`
	TvDueDate          int      `json:"tv_due_date"`
}

type Wallet struct {
	Mid           int `json:"mid"`
	BcoinBalance  int `json:"bcoin_balance"`
	CouponBalance int `json:"coupon_balance"`
	CouponDueTime int `json:"coupon_due_time"`
}

type WbiImg struct {
	ImgUrl string `json:"img_url"`
	SubUrl string `json:"sub_url"`
}

type Downloader struct {
	io.Reader
	Total   int64
	Current int64
}

type Mp4VideoRespData struct {
	From              string      `json:"from"`
	Result            string      `json:"result"`
	Message           string      `json:"message"`
	Quality           int         `json:"quality"`
	Format            string      `json:"format"`
	Timelength        int         `json:"timelength"`
	AcceptFormat      string      `json:"accept_format"`
	AcceptDescription []string    `json:"accept_description"`
	AcceptQuality     []int       `json:"accept_quality"`
	VideoCodecid      int         `json:"video_codecid"`
	SeekParam         string      `json:"seek_param"`
	SeekType          string      `json:"seek_type"`
	Durl              []Durl      `json:"durl"`
	SupportFormats    []Format    `json:"support_formats"`
	HighFormat        interface{} `json:"high_format"`
	LastPlayTime      int         `json:"last_play_time"`
	LastPlayCid       int         `json:"last_play_cid"`
}

type Durl struct {
	Order     int      `json:"order"`
	Length    int      `json:"length"`
	Size      int      `json:"size"`
	Ahead     string   `json:"ahead"`
	Vhead     string   `json:"vhead"`
	Url       string   `json:"url"`
	BackupUrl []string `json:"backup_url"`
}

type Format struct {
	Quality        int         `json:"quality"`
	Format         string      `json:"format"`
	NewDescription string      `json:"new_description"`
	DisplayDesc    string      `json:"display_desc"`
	Superscript    string      `json:"superscript"`
	Codecs         interface{} `json:"codecs"`
}

func (d *Downloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	d.Current += int64(n)
	return
}
