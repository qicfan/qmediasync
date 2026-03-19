package helpers

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	AppLogger = &QLogger{
		Logger: log.New(os.Stdout, "", 0),
	}
	code := m.Run()
	os.Exit(code)
}

type TestCase struct {
	filename          string
	expectedMediaInfo *MediaInfo
}

type TestCases []TestCase

func TestExtractMediaInfoRe_Movie(t *testing.T) {
	testCases := TestCases{
		{
			filename: "【悠哈璃羽字幕社】[死神千年血战相克谭_Bleach - Thousand-Year Blood War - Soukoku Tan][11][1080p][CHT] [432.3 MB]",
			expectedMediaInfo: &MediaInfo{
				Name:    "死神千年血战相克谭",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "【诸神字幕组】[鬼灭之刃_Kimetsu no Yaiba][24][1080p][MP4].mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "鬼灭之刃",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "长安的荔枝[国语配音+中文字幕].The.Lychee.Road.2025.1080p.WEB-DL.H264.AAC-PandaQT",
			expectedMediaInfo: &MediaInfo{
				Name:    "长安的荔枝",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "星际穿越[IMAX满屏版][国英多音轨+简繁英字幕].Interstellar.2014.IMAX.2160p.BluRay.x265.10bit.TrueHD5.1-CTRLHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "星际穿越",
				Year:    2014,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Interstellar.2014.UHD.BluRay.2160p.DTS-HD.MA.5.1.HEVC.REMUX-FraMeSToR",
			expectedMediaInfo: &MediaInfo{
				Name:    "interstellar",
				Year:    2014,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "星际穿越[国英多音轨+中文字幕+特效字幕].Interstellar.2014.2160p.UHD.BluRay.REMUX.HEVC.HDR.DTS-HDMA5.1-DreamHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "星际穿越",
				Year:    2014,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[DBD-Raws][死神/Bleach][OVA][01-02合集][HEVC-10bit][简繁外挂][FLAC][MKV]",
			expectedMediaInfo: &MediaInfo{
				Name:    "死神",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[RU]Caught.Stealing.2025.1080p.MA.WEB-DL.ExKinoRay.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "caught stealing",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Caught.Stealing.2025.MULTi.VF2.2160p.HDR.DV.WEB-DL.H265.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "caught stealing",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "机动战士高达：跨时之战.1080p.HD中字.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "机动战士高达：跨时之战",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "机动战士高达：跨时之战[国语配音+中文字幕].2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO",
			expectedMediaInfo: &MediaInfo{
				Name:    "机动战士高达：跨时之战",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "UIndex - Hans.Zimmer.and.Friends.Diamond.in.the.Desert.2025.1080p.WEB.h264-WEBLE",
			expectedMediaInfo: &MediaInfo{
				Name:    "hans zimmer and friends diamond in the desert",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Hans Zimmer Friends Diamond In The Desert (2025) [720p] [WEBRip] [YTS.MX]",
			expectedMediaInfo: &MediaInfo{
				Name:    "hans zimmer friends diamond in the desert",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "孤独的美食家.剧场版[中文字幕].The.Solitary.Gourmet.2024.1080p.HamiVideo.WEB-DL.AAC2.0.H.264-DreamHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "孤独的美食家",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "戏台.2160p高码版.60fps.HD国语中字无水印.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "戏台",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "戏台[120帧率版本][国语配音+中文字幕].The.Stage.2025.2160p.WEB-DL.H265.HDR.120fps.DTS5.1-DreamHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "戏台",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "戏台[杜比视界版本][高码版][国语配音+中文字幕].The.Stage.2025.2160p.HQ.WEB-DL.H265.DV.DTS5.1-DreamHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "戏台",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The.Stage.2025.WEB.1080p.AC3.Audio.x265-112114119",
			expectedMediaInfo: &MediaInfo{
				Name:    "the stage",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[戏台].The.Stage.2025.2160p.WEB-DL.H265.AAC-CMCTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "the stage",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "《戏台 (2025)》｜4KHDR片源｜黄渤新片｜中字畅享版",
			expectedMediaInfo: &MediaInfo{
				Name:    "戏台",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "【戏台 (2025)】【4K+1080P】【国语中字。】【类型：剧情】 【▶️4K精品影视/_▶️】 ✅✅",
			expectedMediaInfo: &MediaInfo{
				Name:    "戏台",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[60帧率版本][国语配音+中文字幕].The.Stage.2025.2160p.WEB-DL.H265.HDR.60fps.AAC-PandaQT.torrent",
			expectedMediaInfo: &MediaInfo{
				Name:    "the stage",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[HK][东邪西毒.终极版.Ashes.Of.Time.Redux.2008][日版.1080p.REMUX]国粤配][srt.ass简英字幕.sup简繁][30G]",
			expectedMediaInfo: &MediaInfo{
				Name:    "东邪西毒",
				Year:    2008,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "One And Only 2023 HDTV 1080i MP2 H.264-TPTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "one and only",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bone Collector 1999 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bone collector",
				Year:    1999,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bone Collector 1999 BluRay 1080p AVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bone collector",
				Year:    1999,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bad Guys 2 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bad guys 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bad Guys 2 2025 UHD BluRay 2160p HEVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bad guys 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Longest Nite 1998 BluRay 1080p AVC TrueHD 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the longest nite",
				Year:    1998,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Volunteers To the War 2023 BluRay 1080p AVC  DD5.1 -MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the volunteers to the war",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Battle of Life and Death 2024 BluRay 1080p AVC DD5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the battle of life and death",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Happyend 2024 BluRay 1080p AVC TrueHD 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "happyend",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Nobody 2 2025 UHD BluRay 2160p HEVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "nobody 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Nobody 2 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "nobody 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Life of Chuck 2024 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the life of chuck",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "F1 The Movie 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "f1 the movie",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Last Mile 2024 1080p BluRay REMUX AVC DTS-HD MA 5.1-SupaHacka",
			expectedMediaInfo: &MediaInfo{
				Name:    "last mile",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Love of Siam 2007 REMUX 1080p Blu-ray AVC DTS-HD MA 5.1-c0kE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the love of siam",
				Year:    2007,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "One And Only 2023 HDTV 1080i MP2 H.264-TPTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "one and only",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bone Collector 1999 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bone collector",
				Year:    1999,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bone Collector 1999 BluRay 1080p AVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bone collector",
				Year:    1999,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bad Guys 2 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bad guys 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bad Guys 2 2025 UHD BluRay 2160p HEVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bad guys 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Longest Nite 1998 BluRay 1080p AVC TrueHD 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the longest nite",
				Year:    1998,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Volunteers To the War 2023 BluRay 1080p AVC  DD5.1 -MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the volunteers to the war",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Battle of Life and Death 2024 BluRay 1080p AVC DD5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the battle of life and death",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Happyend 2024 BluRay 1080p AVC TrueHD 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "happyend",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Nobody 2 2025 UHD BluRay 2160p HEVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "nobody 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Nobody 2 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "nobody 2",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Life of Chuck 2024 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the life of chuck",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "F1 The Movie 2025 BluRay 1080p AVC Atmos TrueHD7.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "f1 the movie",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Last Mile 2024 1080p BluRay REMUX AVC DTS-HD MA 5.1-SupaHacka",
			expectedMediaInfo: &MediaInfo{
				Name:    "last mile",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Love of Siam 2007 REMUX 1080p Blu-ray AVC DTS-HD MA 5.1-c0kE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the love of siam",
				Year:    2007,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Yeogo goedam 5 Dong ban ja sal AKA A Blood Pledge AKA Whispering Corridors 5 Suicide Pact 2009 DVD5 Remux 480i MPEG-2 DTS",
			expectedMediaInfo: &MediaInfo{
				Name:    "yeogo goedam 5 dong ban ja sal aka a blood pledge aka whispering corridors 5 suicide pact",
				Year:    2009,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Acts of Violence 2018 1080p Blu-ray AVC DTS-HD MA 5.1-Huan@HDSky",
			expectedMediaInfo: &MediaInfo{
				Name:    "acts of violence",
				Year:    2018,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Conjuring: Last Rites 2025 Hybrid 2160p MA WEB-DL DDP 5.1 Atmos DV HDR H.265-HONE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the conjuring: last rites",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Sirāt 2025 2160p MVSTP WEB-DL DD+5.1 HDR H265-HDZ",
			expectedMediaInfo: &MediaInfo{
				Name:    "sirāt",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Crank 2006 GER Extended Cut BluRay 2160p DTS-HDMA5.1 DoVi HDR10 x265 10bit-CHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "crank",
				Year:    2006,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Do the Right Thing 1989 2160p WEB-DL H.264 AAC 2.0-CSWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "do the right thing",
				Year:    1989,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Boys Next Door 1985 2160p UHD Blu-ray HDR10 HEVC DTS-HD MA 5.1-BLoz",
			expectedMediaInfo: &MediaInfo{
				Name:    "the boys next door",
				Year:    1985,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Symphonie pour un massacre 1963 1080p BluRay x264 FLAC 2.0 2Audio-ADE",
			expectedMediaInfo: &MediaInfo{
				Name:    "symphonie pour un massacre",
				Year:    1963,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Warfare 2025 BluRay 2160p TrueHD7.1 DoVi HDR x265 10bit-CHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "warfare",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Great Dictator 1940 1080p CC BluRay Remux AVC FLAC 1.0-ADE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the great dictator",
				Year:    1940,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Six Assassins 1971 USA Blu-ray 1080p AVC DTS-HD MA 2.0-DIY@Hero",
			expectedMediaInfo: &MediaInfo{
				Name:    "six assassins",
				Year:    1971,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Core 2003 2160p UHD Blu-ray DoVi HDR10 HEVC DTS-HD MA 5.1",
			expectedMediaInfo: &MediaInfo{
				Name:    "the core",
				Year:    2003,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Chinese Feast 1995 1080p BluRay Remux AVC TrueHD 5.1 2Audio-ADE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the chinese feast",
				Year:    1995,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Fastest Sword 1968 USA Blu-ray 1080p AVC DTS-HD MA 2.0-DIY@Hero",
			expectedMediaInfo: &MediaInfo{
				Name:    "the fastest sword",
				Year:    1968,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Christine 1958 1080p AMZN WEB-DL H.264 DDP 2.0-SPWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "christine",
				Year:    1958,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Altered States 1980 1080p USA Blu-ray AVC DTS-HD MA 2.0 3Audio-TMT",
			expectedMediaInfo: &MediaInfo{
				Name:    "altered states",
				Year:    1980,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Moonrise Kingdom 2012 2160p UHD Blu-ray Remux DV HEVC DTS-HD MA5.1-HDS",
			expectedMediaInfo: &MediaInfo{
				Name:    "moonrise kingdom",
				Year:    2012,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Jurassic World Rebirth 2025 2160p GER UHD Blu-ray HEVC Atmos TrueHD7.1-HDH",
			expectedMediaInfo: &MediaInfo{
				Name:    "jurassic world rebirth",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Sinners 2025 2160p EUR UHD Blu-ray HEVC Atmos TrueHD7.1-HDH",
			expectedMediaInfo: &MediaInfo{
				Name:    "sinners",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Black Sunday AKA La maschera del demonio 1960 1080p Blu-ray AVC DTS-HD MA 2.0 5Audio-INCUBO",
			expectedMediaInfo: &MediaInfo{
				Name:    "black sunday aka la maschera del demonio",
				Year:    1960,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Thieves Like Us 1974 Blu-ray 1080p AVC DTS-HD MA 2.0-GMA",
			expectedMediaInfo: &MediaInfo{
				Name:    "thieves like us",
				Year:    1974,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Paddington 2 2017 HDTV 1080i MP2 H.264-TPTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "paddington 2",
				Year:    2017,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The NeverEnding Story II The Next Chapter 1990 2160p UHD Blu-ray DoVi HDR10 HEVC DTS-HD MA 5.1 8Audio-DIY@HDSky",
			expectedMediaInfo: &MediaInfo{
				Name:    "the neverending story ii the next chapter",
				Year:    1990,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Rocky Horror Picture Show 1975 2160p UHD Blu-ray DoVi HDR10 HEVC TrueHD 7.1-TMT",
			expectedMediaInfo: &MediaInfo{
				Name:    "the rocky horror picture show",
				Year:    1975,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "G I  Joe Retaliation 2013 2160p UHD Blu-ray DV TrueHD 7.1 3Audio x265-HDH",
			expectedMediaInfo: &MediaInfo{
				Name:    "g i joe retaliation",
				Year:    2013,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Le Havre 2011 1080p BluRay DTS-HD MA 5.1 x264-HDH",
			expectedMediaInfo: &MediaInfo{
				Name:    "le havre",
				Year:    2011,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Russendisko 2012 1080p GER Blu-ray AVC DTS-HD MA 5.1-SharpHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "russendisko",
				Year:    2012,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Departed 2006 1080p Blu-ray AVC DTS-HD MA 5.1 2Audio-NoGrp",
			expectedMediaInfo: &MediaInfo{
				Name:    "the departed",
				Year:    2006,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Bone Collector 1999 2160p UHD Blu-ray HEVC DTS-HD MA5.1-HDH",
			expectedMediaInfo: &MediaInfo{
				Name:    "the bone collector",
				Year:    1999,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Anora 2024 2160p BluRay HDR10+ x265 DTS-HD MA 5.1 3Audio-MainFrame",
			expectedMediaInfo: &MediaInfo{
				Name:    "anora",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Hunt 2020 BluRay 1080p x265 10bit DDP7.1 MNHD-FRDS",
			expectedMediaInfo: &MediaInfo{
				Name:    "the hunt",
				Year:    2020,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Family 2021 1080p GER Blu-ray MPEG-2 DTS-HD MA 5.1-SharpHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "the family",
				Year:    2021,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "L Ultima Volta Che Siamo Stati Bambini 2023 BluRay 1080p x265 10bit DDP5.1 MNHD-FRDS",
			expectedMediaInfo: &MediaInfo{
				Name:    "l ultima volta che siamo stati bambini",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Creed II 2018 USA BluRay 2160p TrueHD7.1 DoVi HDR10 x265 10bit-CHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "creed ii",
				Year:    2018,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Smile 2022 UHD Bluray 2160p DV HDR x265 10bit Atmos TrueHD 7.1 2Audio-UBits",
			expectedMediaInfo: &MediaInfo{
				Name:    "smile",
				Year:    2022,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "FF9 2021 2160p HQ 60fps WEB-DL H.265 HDR AAC 2.0 2Audio-ZmWeb",
			expectedMediaInfo: &MediaInfo{
				Name:    "ff9",
				Year:    2021,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Weird Man 1983 1080p Blu-ray AVC LPCM 2.0-MKu",
			expectedMediaInfo: &MediaInfo{
				Name:    "the weird man",
				Year:    1983,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Wind Is Blowing 2020 HDTV 1080i MP2 H.264-TPTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "the wind is blowing",
				Year:    2020,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Running Man 1987 2160p FRA UHD Blu-ray DV HDR HEVC DTS-HD MA 5.1-DIY@HDSky",
			expectedMediaInfo: &MediaInfo{
				Name:    "the running man",
				Year:    1987,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Dongji Island 2025 2160p HQ WEB-DL H.265 10bit HDR DoVi DDP 5.1-CMCTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "dongji island",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Ruth and Boaz 2025 2160p NF WEB-DL DV H.265 DDP5.1 Atmos-ADWeb",
			expectedMediaInfo: &MediaInfo{
				Name:    "ruth and boaz",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Uranus 2324 2024 2160p friDay WEB-DL H.265 AAC 2.0-UBWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "uranus 2324",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "I Swear 2025 2160p HQ WEB-DL H.265 10bit HDR DoVi DDP 5.1-CMCTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "i swear",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "F1 The Movie 2025 2160p UHD BluRay x265 10bit DV HDR10 TrueHD 7.1 Atmos-Panda",
			expectedMediaInfo: &MediaInfo{
				Name:    "f1 the movie",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "2:37 2006 EUR BluRay AVC LPCM  2Audio-TYZH@HDSky",
			expectedMediaInfo: &MediaInfo{
				Name:    "2:37",
				Year:    2006,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Jolly Monkey 2025 1080p Blu-ray AVC DTS-HD MA 5.1-iFPD",
			expectedMediaInfo: &MediaInfo{
				Name:    "the jolly monkey",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Sinners 2025 USA BluRay Remux AVC 1080p Atmos TrueHD7.1-CHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "sinners",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Man in Black 1950 2160p UHD Blu-ray DoVi HDR10 HEVC DTS-HD MA 5.1-LWRTD",
			expectedMediaInfo: &MediaInfo{
				Name:    "the man in black",
				Year:    1950,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Life of Chuck 2024 BluRay 2160p HDR x265 DTS-HD MA 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the life of chuck",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Shimoni 2022 1080p WEB-DL AAC2.0 x264-ZTR",
			expectedMediaInfo: &MediaInfo{
				Name:    "shimoni",
				Year:    2022,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Conjuring Last Rites 2025 2160p iTunes WEB-DL DDP 5.1 Atmos DV H.265-CHDWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "the conjuring last rites",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Conjuring Last Rites 2025 2160p iTunes WEB-DL DDP 5.1 Atmos H.265-CHDWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "the conjuring last rites",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Fetus 2025 1080p Blu-ray AVC DTS-HD MA 5.1-PtBM",
			expectedMediaInfo: &MediaInfo{
				Name:    "the fetus",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Fantastic Four: First Steps 2025 2160p BluRay DoVi x265 10bit 3Audios TrueHD Atmos 7.1-WiKi",
			expectedMediaInfo: &MediaInfo{
				Name:    "the fantastic four: first steps",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Life of Chuck 2024 BluRay 1080p x265 DTS-HD MA 5.1-MTeam",
			expectedMediaInfo: &MediaInfo{
				Name:    "the life of chuck",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Habit 2021 BluRay 1080p x265 10bit DDP5.1 MNHD-FRDS",
			expectedMediaInfo: &MediaInfo{
				Name:    "habit",
				Year:    2021,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Million Eyes of Sumuru 1967 2160p UHD Blu-ray DoVi HDR10 HEVC DD 2.0-DIY@HDSky",
			expectedMediaInfo: &MediaInfo{
				Name:    "the million eyes of sumuru",
				Year:    1967,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Past 2013 USA 1080p Blu-ray AVC DTS-HD MA 5.1-blucook#792@CHDBits",
			expectedMediaInfo: &MediaInfo{
				Name:    "the past",
				Year:    2013,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Smiles of a Summer Night 1955 BFI Blu-ray 1080p AVC LPCM 1.0-blucook#344@CHDBits",
			expectedMediaInfo: &MediaInfo{
				Name:    "smiles of a summer night",
				Year:    1955,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Conjuring Last Rites 2025 2160p iTunes WEB-DL DDP 5.1 Atmos HDR10+ H.265-CHDWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "the conjuring last rites",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Naked City 1948 1080p BluRay AVC LPCM 1.0 2Audio-DiY@HDHome",
			expectedMediaInfo: &MediaInfo{
				Name:    "the naked city",
				Year:    1948,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Tuche Family 2011 1080p FRA Blu-ray VC-1 DTS-HD MA 5.1-F13@HDSpace",
			expectedMediaInfo: &MediaInfo{
				Name:    "the tuche family",
				Year:    2011,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Conjuring Last Rites 2025 1080p WEB-DL HEVC x265 5.1 BONE",
			expectedMediaInfo: &MediaInfo{
				Name:    "the conjuring last rites",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Litchi Road 2025 2160p WEB-DL H.265 AAC 2.0-CMCTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "the litchi road",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Dongji Island 2025 2160p WEB-DL H.265 AAC 2.0-CMCTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "dongji island",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Noise 2023 1080p NF WEB-DL DDP5.1 Atmos H.264-HHWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "noise",
				Year:    2023,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "For Our Pure Time 2021 2160p WEB-DL H.265 DDP 2.0 2Audio-HHWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "for our pure time",
				Year:    2021,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Way of the Househusband: The Cinema 2022 2160p WEB-DL H.264 AAC 2.0 2Audio-CSWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "the way of the househusband: the cinema",
				Year:    2022,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Dog Days 2018 1080p GER Blu-ray AVC DTS-HD MA 5.1.2Audios-PTer",
			expectedMediaInfo: &MediaInfo{
				Name:    "dog days",
				Year:    2018,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Lv Jian Jiang 2020 2160p WEB-DL H.265 DDP 2.0 2Audio5.1-HHWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "lv jian jiang",
				Year:    2020,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Happyend 2024 BluRay 1080p x265 10bit DDP5.1 MNHD-FRDS",
			expectedMediaInfo: &MediaInfo{
				Name:    "happyend",
				Year:    2024,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Primal Fear 1996 2160p NF WEB-DL DV H.265 DDP 5.1-CHDWEB",
			expectedMediaInfo: &MediaInfo{
				Name:    "primal fear",
				Year:    1996,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The Black Tulip 1964 HDTV 1080i AAC2.0 H.264-TPTV",
			expectedMediaInfo: &MediaInfo{
				Name:    "the black tulip",
				Year:    1964,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "FF9 2021 2160p HQ WEB-DL H.265 DV AAC 2.0 2Audio-ZmWeb",
			expectedMediaInfo: &MediaInfo{
				Name:    "ff9",
				Year:    2021,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Matt McCusker A Humble Offering 2025 1080p NF WEB-DL DDP5.1 H.264-MWeb",
			expectedMediaInfo: &MediaInfo{
				Name:    "matt mccusker a humble offering",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "[黒ネズミたち] 妖怪旅馆营业中 贰 / Kakuriyo no Yadomeshi Ni - 01 (CR 1920x1080 AVC AAC MKV)[1080P]",
			expectedMediaInfo: &MediaInfo{
				Name:    "妖怪旅馆营业中 贰",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "Steve.2025.1080p.NF.WEB-DL.DDP.5.1.Atmos.H.264-NukeHD",
			expectedMediaInfo: &MediaInfo{
				Name:    "steve",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "caught stealing (2025) - 1440p",
			expectedMediaInfo: &MediaInfo{
				Name:    "caught stealing",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The.Phantom.Lover.1995.BluRay.1080p.TrueHD5.1.x265.10bit-Xiaomi.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "the phantom lover",
				Year:    1995,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The.Party.2.1982.FRE.REMASTERED.1080p.BluRay.REMUX.AVC.DTS-HD.MA.2.0.DDP2.1.CUSTOM-Asmo.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "the party 2",
				Year:    1982,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "《霸王别姬》1993.国粤双语版",
			expectedMediaInfo: &MediaInfo{
				Name:    "霸王别姬",
				Year:    1993,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "36.《甜蜜的事业》1979.超清修复版.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "甜蜜的事业",
				Year:    1979,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "37《瞧这一家子》1979.超清修复版.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "瞧这一家子",
				Year:    1979,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "11.《太太万岁》1947.老片修复版.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "太太万岁",
				Year:    1947,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "14、旋风小子 CCTV6-HD【公众号：手机软件资源局】.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "旋风小子",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "13、黄河绝恋CCTV6-HD【公众号：手机软件资源局】.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "黄河绝恋",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The.Myth.2005.BluRay.1080p.x265.10bit.2Audios.MNHD-FRDS.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "the myth",
				Year:    2005,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "笔仙惊魂3 - NGB.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "笔仙惊魂3",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "The.Top.Bet.1991.1080p.BluRay.REMUX.AVC.PCM.2.0.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "the top bet",
				Year:    1991,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "寻找CCP - NGB.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "寻找CCP",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "飞狐外传－NGB.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "飞狐外传",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "豪侠传－NGB.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "豪侠传",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "火线干探 复仇 德国.strm",
			expectedMediaInfo: &MediaInfo{
				Name:    "火线干探",
				Year:    0,
				Season:  -1,
				Episode: 0,
			},
		},
		{
			filename: "创：战神_Tron: Ares【2025_tt6604188】.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "创：战神",
				Year:    2025,
				Season:  -1,
				Episode: 0,
			},
		},
	}
	i := 1
	for _, tc := range testCases {
		info := ExtractMediaInfoRe(tc.filename, true, false, []string{".strm", ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".avi", ".ts", ".m4v", ".iso"})
		if info == nil {
			t.Fatalf("正则提取视频信息失败： '%s'", tc.filename)
			continue
		}
		// 验证函数能够正常工作，并且返回的MediaInfo结构有效
		if !strings.EqualFold(info.Name, tc.expectedMediaInfo.Name) {
			t.Errorf("正则提取视频信息失败： '%s', 视频名称 '%s' 与预期 '%s' 不符", tc.filename, info.Name, tc.expectedMediaInfo.Name)
			continue
		}
		if info.Year != tc.expectedMediaInfo.Year {
			t.Errorf("正则提取视频信息失败： '%s', 视频年份 %d 与预期 %d 不符", tc.filename, info.Year, tc.expectedMediaInfo.Year)
			continue
		}
		// if info.Season != tc.expectedMediaInfo.Season {
		// 	t.Errorf("正则提取视频信息失败： '%s', 视频季数 %d 与预期 %d 不符", tc.filename, info.Season, tc.expectedMediaInfo.Season)
		// 	continue
		// }
		// if info.Episode != tc.expectedMediaInfo.Episode {
		// 	t.Errorf("正则提取视频信息失败： '%s', 视频集数 %d 与预期 %d 不符", tc.filename, info.Episode, tc.expectedMediaInfo.Episode)
		// 	continue
		// }
		i++
	}
	fmt.Printf("共测试完成 %d 个电影标题\n", i)
}

func TestExtractMediaInfoRe_Tvshow(t *testing.T) {
	testCases := TestCases{
		{
			filename: "【漫游字幕组】[进击的巨人_Attack on Titan][S04E16][1080p][CHS].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "进击的巨人",
				Year:    0,
				Season:  4,
				Episode: 16,
			},
		},
		{
			filename: "【银色子弹字幕组】[名侦探柯南][第74集 死神阵内杀人事件][WEBRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]",
			expectedMediaInfo: &MediaInfo{
				Name:    "名侦探柯南",
				Year:    0,
				Season:  -1,
				Episode: 74,
			},
		},
		{
			filename: "人民的名义.S01E34.利剑行动开始.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "人民的名义",
				Year:    0,
				Season:  1,
				Episode: 34,
			},
		},
		{
			filename: "棋士.Playing.Go.S01E01.2025.2160p.WEB-DL.H265.DV.DDP5.1.Atmos.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "棋士",
				Year:    2025,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "知否知否应是绿肥红瘦 66.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "知否知否应是绿肥红瘦",
				Year:    0,
				Season:  -1,
				Episode: 66,
			},
		},
		{
			filename: "66.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "",
				Year:    0,
				Season:  -1,
				Episode: 66,
			},
		},
		{
			filename: "[LoliHouse] Mattaku Saikin no Tantei to Kitara - 10 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "mattaku saikin no tantei to kitara",
				Year:    0,
				Season:  -1,
				Episode: 10,
			},
		},
		{
			filename: "[LoliHouse] Mattaku Saikin no Tantei to Kitara - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "mattaku saikin no tantei to kitara",
				Year:    0,
				Season:  -1,
				Episode: 1,
			},
		},
		{
			filename: "日久见君心.Time.Reveals.the.Prince.s.Heart.S01E01.2025.2160p.WEB-DL.H265.AAC",
			expectedMediaInfo: &MediaInfo{
				Name:    "日久见君心",
				Year:    2025,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "Confidence.Queen.S01E01.1080p.Amazon.WEB-DL.AAC2.0.H.264-COKEMV",
			expectedMediaInfo: &MediaInfo{
				Name:    "confidence queen",
				Year:    0,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "凡人修仙传 - S01E20 - 第 20 集.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "凡人修仙传",
				Year:    0,
				Season:  1,
				Episode: 20,
			},
		},
		{
			filename: "超感猎杀 - S01E08 - 第 8 集.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "超感猎杀",
				Year:    0,
				Season:  1,
				Episode: 8,
			},
		},
		{
			filename: "泰拉玛斯卡.Talamasca.The.Secret.Order.S01E03.Bread.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "泰拉玛斯卡",
				Year:    0,
				Season:  1,
				Episode: 3,
			},
		},
		{
			filename: "猎魔人.The.Witcher.S02E08.1080p.H265-官方中字.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "猎魔人",
				Year:    0,
				Season:  2,
				Episode: 8,
			},
		},
	}
	i := 0
	for _, tc := range testCases {
		i++
		info := ExtractMediaInfoRe(tc.filename, false, false, []string{".strm", ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".avi", ".ts", ".m4v", ".iso"})
		if info == nil {
			t.Fatalf("正则提取视频信息失败： '%s'", tc.filename)
			continue
		}
		// 验证函数能够正常工作，并且返回的MediaInfo结构有效
		if !strings.EqualFold(info.Name, tc.expectedMediaInfo.Name) {
			t.Errorf("正则提取视频信息失败： '%s', 视频名称 '%s' 与预期 '%s' 不符", tc.filename, info.Name, tc.expectedMediaInfo.Name)
			continue
		}
		if info.Year != tc.expectedMediaInfo.Year {
			t.Errorf("正则提取视频信息失败： '%s', 视频年份 %d 与预期 %d 不符", tc.filename, info.Year, tc.expectedMediaInfo.Year)
			continue
		}
		if info.Season != tc.expectedMediaInfo.Season {
			t.Errorf("正则提取视频信息失败： '%s', 视频季数 %d 与预期 %d 不符", tc.filename, info.Season, tc.expectedMediaInfo.Season)
			continue
		}
		if info.Episode != tc.expectedMediaInfo.Episode {
			t.Errorf("正则提取视频信息失败： '%s', 视频集数 %d 与预期 %d 不符", tc.filename, info.Episode, tc.expectedMediaInfo.Episode)
			continue
		}
		// fmt.Printf("正则提取视频信息成功： '%s', 影视剧名称：%s, 年份：%d, 季: %d, 集：%d\n\n", tc.filename, info.Name, info.Year, info.Season, info.Episode)
	}
	fmt.Printf("共测试完成 %d 个电视剧标题\n", i)
}

func TestExtractMediaInfoRe_TvshowSeasonEpisode(t *testing.T) {
	testCases := TestCases{
		{
			filename: "【漫游字幕组】[进击的巨人_Attack on Titan][S04E16][1080p][CHS].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "进击的巨人",
				Year:    0,
				Season:  4,
				Episode: 16,
			},
		},
		{
			filename: "【银色子弹字幕组】[名侦探柯南][第74集 死神阵内杀人事件][WEBRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]",
			expectedMediaInfo: &MediaInfo{
				Name:    "名侦探柯南",
				Year:    0,
				Season:  -1,
				Episode: 74,
			},
		},
		{
			filename: "人民的名义.S01E34.利剑行动开始.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "人民的名义",
				Year:    0,
				Season:  1,
				Episode: 34,
			},
		},
		{
			filename: "棋士.Playing.Go.S01E01.2025.2160p.WEB-DL.H265.DV.DDP5.1.Atmos.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "棋士",
				Year:    2025,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "知否知否应是绿肥红瘦 66.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "知否知否应是绿肥红瘦",
				Year:    0,
				Season:  -1,
				Episode: 66,
			},
		},
		{
			filename: "66.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "",
				Year:    0,
				Season:  -1,
				Episode: 66,
			},
		},
		{
			filename: "[LoliHouse] Mattaku Saikin no Tantei to Kitara - 10 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "mattaku saikin no tantei to kitara",
				Year:    0,
				Season:  -1,
				Episode: 10,
			},
		},
		{
			filename: "[LoliHouse] Mattaku Saikin no Tantei to Kitara - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "mattaku saikin no tantei to kitara",
				Year:    0,
				Season:  -1,
				Episode: 1,
			},
		},
		{
			filename: "棋士.Playing.Go.S01E20.2025.2160p.WEB-DL DV.H265.DDP 5.1 Atmos.mp4",
			expectedMediaInfo: &MediaInfo{
				Name:    "棋士",
				Year:    2025,
				Season:  1,
				Episode: 20,
			},
		},
		{
			filename: "日久见君心.Time.Reveals.the.Prince.s.Heart.S01E01.2025.2160p.WEB-DL.H265.AAC",
			expectedMediaInfo: &MediaInfo{
				Name:    "日久见君心",
				Year:    2025,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "Confidence.Queen.S01E01.1080p.Amazon.WEB-DL.AAC2.0.H.264-COKEMV",
			expectedMediaInfo: &MediaInfo{
				Name:    "Confidence.Queen",
				Year:    0,
				Season:  1,
				Episode: 1,
			},
		},
		{
			filename: "凡人修仙传 - S01E20 - 第 20 集.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "凡人修仙传",
				Year:    0,
				Season:  1,
				Episode: 20,
			},
		},
		{
			filename: "超感猎杀 - S01E08 - 第 8 集.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "超感猎杀",
				Year:    0,
				Season:  1,
				Episode: 8,
			},
		},
		{
			filename: "超感猎杀 - S2EP10 - 第 10 集.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "超感猎杀",
				Year:    0,
				Season:  2,
				Episode: 10,
			},
		},
		{
			filename: "[VCB-Studio] Shiunji-ke no Kodomotachi [01][Ma10p_1080p][x265_flac].mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "Shiunji-ke no Kodomotachi",
				Year:    0,
				Season:  -1,
				Episode: 1,
			},
		},
		{
			filename: "01 4K.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "Shiunji-ke no Kodomotachi",
				Year:    0,
				Season:  -1,
				Episode: 1,
			},
		},
		{
			filename: "01.mkv",
			expectedMediaInfo: &MediaInfo{
				Name:    "Shiunji-ke no Kodomotachi",
				Year:    0,
				Season:  -1,
				Episode: 1,
			},
		},
	}
	i := 0
	for _, tc := range testCases {
		i++
		info := ExtractMediaInfoRe(tc.filename, false, true, []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".avi", ".ts", ".m4v", ".iso"})
		if info == nil {
			t.Fatalf("正则提取视频信息失败： '%s'", tc.filename)
			continue
		}
		if info.Season != tc.expectedMediaInfo.Season {
			t.Errorf("正则提取视频信息失败： '%s', 视频季数 %d 与预期 %d 不符", tc.filename, info.Season, tc.expectedMediaInfo.Season)
			continue
		}
		if info.Episode != tc.expectedMediaInfo.Episode {
			t.Errorf("正则提取视频信息失败： '%s', 视频集数 %d 与预期 %d 不符", tc.filename, info.Episode, tc.expectedMediaInfo.Episode)
			continue
		}
	}
	fmt.Printf("共测试完成 %d 个电视剧季集\n", i)
}

func TestExtractMediaInfoRe_TvshowSeason(t *testing.T) {
	testCases := TestCases{
		{
			filename: "S01",
			expectedMediaInfo: &MediaInfo{
				Name:    "",
				Year:    0,
				Season:  1,
				Episode: -1,
			},
		},
	}
	i := 0
	for _, tc := range testCases {
		i++
		info := ExtractSeasonsFromSeasonPath(tc.filename)
		if info != tc.expectedMediaInfo.Season {
			t.Errorf("正则提取视频信息失败： '%s', 视频季数 %d 与预期 %d 不符", tc.filename, info, tc.expectedMediaInfo.Season)
			continue
		}
	}
	fmt.Printf("共测试完成 %d 个电视剧季集\n", i)
}

func TestExtractMediaInfoRe_Tmdb(t *testing.T) {
	testCases := TestCases{
		{
			filename: "凡人修仙传 (2025) {tmdbid-12312312}",
			expectedMediaInfo: &MediaInfo{
				Name:   "凡人修仙传",
				Year:   2025,
				TmdbId: 12312312,
			},
		},
		{
			filename: "凡人修仙传 (2025) [tmdbid-12312312]",
			expectedMediaInfo: &MediaInfo{
				Name:   "凡人修仙传",
				Year:   2025,
				TmdbId: 12312312,
			},
		},
	}
	i := 0
	for _, tc := range testCases {
		i++
		info := ExtractMediaInfoRe(tc.filename, false, true, []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".avi", ".ts", ".m4v", ".iso"})
		if info == nil {
			t.Fatalf("正则提取视频信息失败： '%s'", tc.filename)
			continue
		}
		if info.TmdbId != tc.expectedMediaInfo.TmdbId {
			t.Errorf("正则提取视频信息失败： '%s', 视频tmdb id %d 与预期 %d 不符", tc.filename, info.TmdbId, tc.expectedMediaInfo.TmdbId)
			continue
		}
	}
}
