package model

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/icrowley/fake"
	"github.com/logica0419/gasshuku-isucon/bench/utils"
	"github.com/mattn/go-gimei"
)

type Genre int

const (
	General         Genre = iota // 総記
	Philosophy                   // 哲学・心理学
	Religion                     // 宗教・神学
	SocialScience                // 社会科学
	Vacant                       // 未定義
	Mathematics                  // 数学・自然科学
	AppliedSciences              // 応用科学・医学・工学
	Arts                         // 芸術
	Literature                   // 言語・文学
	Geography                    // 地理・歴史
)

func (g Genre) String() string {
	return strconv.Itoa(int(g))
}

type Book struct {
	ID        string    `json:"id" db:"id"`
	Title     string    `json:"title" db:"title"`
	Author    string    `json:"author" db:"author"`
	Genre     Genre     `json:"genre" db:"genre"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type BookWithLending struct {
	Book
	Lending bool `json:"lending"`
}

func NewBook() *BookWithLending {
	return &BookWithLending{
		Book: Book{
			ID:        utils.GenerateID(),
			Title:     NewBookTitle(),
			Author:    NewBookAuthor(),
			Genre:     NewBookGenre(),
			CreatedAt: time.Now(),
		},
		Lending: false,
	}
}

func NewBookAuthor() string {
	if rand.Intn(3) == 0 {
		return fake.FullName()
	}
	return gimei.NewName().Kanji()
}

func NewBookGenre() Genre {
	genre, _ := utils.WeightedSelect(
		[]utils.Choice[Genre]{
			{Val: General, Weight: 3},
			{Val: Philosophy, Weight: 2},
			{Val: Religion, Weight: 1},
			{Val: SocialScience, Weight: 2},
			{Val: Mathematics, Weight: 5},
			{Val: AppliedSciences, Weight: 6},
			{Val: Arts, Weight: 1},
			{Val: Literature, Weight: 1},
			{Val: Geography, Weight: 1},
		}, false,
	)
	return genre
}

func NewBookTitle() string {
	if rand.Intn(3) == 0 {
		return strings.TrimSuffix(fake.Sentence(), ".")
	}
	title := ""
	for i := 0; i < rand.Intn(15)+5; i++ {
		title += titleComponentList[rand.Intn(len(titleComponentList))]
	}
	return title
}

var titleComponentList = []string{
	"Ｓカルマ氏の",
	"アカシヤの",
	"あかね",
	"あさくさの",
	"アサッテの",
	"アトラス",
	"あの",
	"アメリカ",
	"アメリカン",
	"イエスの",
	"いつか",
	"エーゲ海に",
	"オキナワの",
	"おどる",
	"おらおらで",
	"お吟",
	"カクテル",
	"カディスの",
	"グランド",
	"ゲルマニウムの",
	"コシャマイン",
	"この人の",
	"されど",
	"しょっぱい",
	"ジョン万次郎",
	"スティル",
	"ソウルミュージック",
	"それぞれの",
	"タイムスリップ",
	"ダイヤモンド",
	"つまを",
	"テロリストの",
	"てんのじ",
	"ナポレオン",
	"ナリン殿下への",
	"ニューギニヤ",
	"ネコババの",
	"はぐれ",
	"ハリガネ",
	"パーク",
	"ビタミン",
	"ひとり",
	"ファースト",
	"ブエノスアイレス",
	"プレオー８の",
	"プールサイド",
	"ベティさんの",
	"ほかならぬ",
	"ホテル",
	"ポトスライムの",
	"ボロ家の",
	"まほろ駅前",
	"マークスの",
	"ミミの",
	"モッキングバードの",
	"やまあいの",
	"ルソンの",
	"愛の",
	"悪い",
	"或る",
	"暗殺の",
	"異類",
	"一絃の",
	"蔭",
	"蔭の",
	"陰気な",
	"雲南",
	"英語屋",
	"演歌の",
	"炎熱",
	"遠い",
	"遠い海から",
	"遠い国からの",
	"王妃の",
	"黄色い",
	"沖で",
	"乙女の",
	"下町",
	"夏の",
	"夏姫",
	"家族",
	"歌と門の",
	"火垂るの",
	"花",
	"花の",
	"過越しの",
	"蛾と",
	"介護",
	"海の",
	"海の見える",
	"海峡の",
	"海人",
	"海狼",
	"兜",
	"蒲田",
	"感傷",
	"雁の",
	"鬼の",
	"吉原",
	"吉野朝",
	"砧を",
	"魚河岸",
	"僑人の",
	"強情",
	"強力",
	"銀河鉄道の",
	"九月の",
	"苦役",
	"愚者の",
	"空中",
	"熊の",
	"軍旗はためく",
	"軍事",
	"恵比寿屋",
	"鶏",
	"鯨",
	"月",
	"月と",
	"月の",
	"犬",
	"肩ごしの",
	"鍵のない",
	"元首の",
	"限りなく",
	"孤愁の",
	"後巷説",
	"御苦労",
	"光と",
	"光抱く",
	"厚物",
	"巷談",
	"広場の",
	"江分利満氏の",
	"高安犬",
	"高円寺",
	"号泣する準備は",
	"黒パン",
	"佐川君からの",
	"最終便に",
	"祭りの",
	"罪な",
	"鷺と",
	"三匹の",
	"山",
	"子育て",
	"志賀",
	"私が殺した",
	"私の",
	"至高",
	"自動",
	"執行",
	"蛇に",
	"蛇を",
	"寂寥",
	"手鎖",
	"受け",
	"秋田口の",
	"終の",
	"終身",
	"蹴りたい",
	"柔らかな",
	"熟れてゆく",
	"春の",
	"女たちの",
	"女の",
	"小さい",
	"小さな",
	"小説",
	"小伝",
	"少年の",
	"昭和の",
	"上総",
	"乗合",
	"新宿",
	"深い",
	"深重の",
	"真説",
	"人間万事",
	"人生の",
	"塵の",
	"尋ね人の",
	"星々の",
	"聖",
	"青",
	"青果の",
	"青玉",
	"青春",
	"石の",
	"赤い",
	"赤頭巾ちゃん",
	"赤目四十八瀧",
	"戦いすんで",
	"総会屋",
	"草の",
	"蒼ざめた",
	"村の",
	"太陽の",
	"対岸の",
	"大浪花",
	"誰かが",
	"端島の",
	"中陰の",
	"張少子の",
	"暢気",
	"長江",
	"長崎",
	"長男の",
	"津軽",
	"佃島",
	"天皇の",
	"天才と狂人の",
	"天正",
	"纏足の",
	"土の",
	"土の中の",
	"凍える",
	"凍れる",
	"悼む",
	"東京新大橋",
	"燈台",
	"道化師の",
	"徳山道助の",
	"豚の",
	"鍋の",
	"二つの",
	"虹の谷の",
	"乳と",
	"妊娠",
	"忍ぶ",
	"年の",
	"馬淵",
	"廃墟に",
	"背徳の",
	"背負い",
	"白い",
	"白球",
	"八月の",
	"緋い",
	"美談の",
	"百年",
	"漂砂の",
	"漂泊者の",
	"表層",
	"父が",
	"武道",
	"風に舞いあがる",
	"風流",
	"復讐するは",
	"糞尿",
	"平賀",
	"壁の",
	"北の",
	"僕って",
	"本の",
	"蜜蜂と",
	"夢の",
	"無間",
	"無明",
	"冥土",
	"明治",
	"猛スピードで",
	"杢二の",
	"夜と霧の",
	"容疑者Ｘの",
	"裸の",
	"利休に",
	"硫黄",
	"虜愁",
	"鈴木",
	"恋忘れ",
	"浪曲師",
	"狼",
	"團十郎",
	"梟の",
	"榧の",
	"螢の",
	"邂逅の",
	"颱風",
	"鶴八",
	"時が",
	"時代屋の",
	"アメリカ",
	"アリア",
	"いくさ",
	"いちご",
	"いる町",
	"いる町で",
	"うたう",
	"うつ女",
	"おうち",
	"カレンダー",
	"かわうそ",
	"きことわ",
	"きれぎれ",
	"ごっこ",
	"こと",
	"コンビナート",
	"コンビニ 人間",
	"さま",
	"サラバ",
	"さん",
	"さんご",
	"シネマ",
	"ジハード",
	"じょんから節",
	"しんせかい",
	"スクラップアンド ビルド",
	"スクール",
	"ダスト",
	"たずねよ",
	"つるぎ",
	"できていた",
	"でく",
	"デルタ",
	"デンデケデケデケ",
	"ドライブ",
	"のれん",
	"パラソル",
	"パーティー",
	"ピアス",
	"ひじき",
	"ひとりいぐも",
	"ビニールシート",
	"フィナーレ",
	"フォーティーン",
	"ふたり書房",
	"プラナリア",
	"ぶらぶら節",
	"ブランコ",
	"まんま",
	"ムシ",
	"めぐり",
	"メス",
	"めとらば",
	"ものがたり",
	"ライフ",
	"ラヴ",
	"ラバーズオンリー",
	"れくいえむ",
	"ロケット",
	"ローヤル",
	"われらが日々",
	"阿呆",
	"異邦人",
	"一代女",
	"雨やどり",
	"雨中図",
	"運転士",
	"影",
	"影裏",
	"炎環",
	"煙",
	"遠雷",
	"王様",
	"下に",
	"何",
	"何者",
	"夏",
	"河",
	"火花",
	"花",
	"我にあり",
	"牙",
	"回想",
	"海",
	"蟹",
	"確証",
	"寛容",
	"漢奸",
	"間",
	"間に合えば",
	"岸",
	"玩具",
	"眼鏡",
	"雁立",
	"喜兵衛手控え",
	"器",
	"機雷",
	"帰郷",
	"気をつけて",
	"汽笛を鳴らして",
	"季節",
	"記",
	"記憶",
	"貴婦人",
	"起床装置",
	"鬼",
	"桔梗",
	"共喰い",
	"橋",
	"狂",
	"桐",
	"錦城",
	"琴",
	"九年前の 祈り",
	"空",
	"隅で",
	"兄弟",
	"穴",
	"月",
	"犬",
	"犬小屋",
	"献身",
	"源内",
	"孤独",
	"五月",
	"午前零時",
	"乞う",
	"光",
	"行進曲",
	"郊野",
	"香港",
	"頃",
	"婚姻譚",
	"塞翁が丙午",
	"祭",
	"咲",
	"錯乱",
	"笹舟",
	"殺人者",
	"鮫",
	"山",
	"山河",
	"山岳戦",
	"山畠",
	"山妣",
	"斬",
	"残り",
	"残映",
	"刺青",
	"子供",
	"市",
	"死んでいない 者",
	"獅子香炉",
	"私生活",
	"詩",
	"寺",
	"主水",
	"守備兵",
	"手引草",
	"手紙",
	"首",
	"終楽章",
	"舟",
	"住処",
	"出家",
	"出発",
	"春の 庭",
	"春秋",
	"盾",
	"純情商店街",
	"女",
	"女合戦",
	"女房",
	"勝烏",
	"商人",
	"小景",
	"小指",
	"小倉日記伝",
	"少女",
	"少年",
	"抄",
	"消えた",
	"城",
	"城外",
	"場",
	"触った",
	"伸予",
	"心中",
	"心中未遂",
	"森",
	"深川唄",
	"人",
	"人へ",
	"人形",
	"水",
	"水滴",
	"世界",
	"世去れ節",
	"棲みか",
	"生きる",
	"生活",
	"聖所",
	"聖水",
	"石川五右衛門",
	"赤い星",
	"切腹事件",
	"切羽へ",
	"雪",
	"川",
	"喪神",
	"草",
	"蒼氓",
	"送り火",
	"騒動",
	"村",
	"多田便利軒",
	"太平記",
	"待つ",
	"大連",
	"谷間",
	"男",
	"地中海",
	"中",
	"仲間",
	"虫",
	"朝日丸の話",
	"蝶",
	"長恨歌",
	"長夜",
	"長良川",
	"追いつめる",
	"爪と 目",
	"庭",
	"泥",
	"鉄道員",
	"伝",
	"伝説",
	"登攀",
	"塔",
	"島",
	"等伯",
	"踏む",
	"透明に近いブルー",
	"闘牛",
	"瞳",
	"虹",
	"日が暮れて",
	"日蝕",
	"日本婦道記",
	"日和",
	"入門",
	"年輪",
	"念仏",
	"破門",
	"馬を見よ",
	"馬車",
	"廃園",
	"背中",
	"八百長",
	"叛乱",
	"犯罪",
	"彼女",
	"秘伝",
	"百物語",
	"漂流記",
	"敷石",
	"普賢",
	"父",
	"腐し",
	"風土記",
	"物語",
	"壁",
	"墓",
	"母は",
	"報い",
	"奉行",
	"捧ぐ",
	"帽子",
	"謀叛",
	"頬",
	"本牧亭",
	"未決囚",
	"岬",
	"密告",
	"密猟者",
	"夢を見る",
	"婿入り",
	"名前",
	"面",
	"木祭り",
	"夜",
	"夜明け",
	"約束",
	"愉しみ",
	"優雅な生活",
	"友よ",
	"猶予",
	"由煕",
	"郵便",
	"夕陽",
	"来たＣＯＯ",
	"来歴",
	"落ちる",
	"卵",
	"理髪店",
	"理由",
	"離婚",
	"劉廣福",
	"流",
	"流れ",
	"旅行",
	"領分",
	"列車",
	"恋",
	"恋歌",
	"恋紅",
	"恋人",
	"恋文",
	"連絡員",
	"路上に捨てる",
	"和紙",
	"話",
	"俘虜記",
	"傳來記",
	"杳子",
	"檻",
	"滲む朝",
	"罌粟",
	"蜩ノ記",
	"螢川",
	"裔",
	"譚",
	"鏨師",
	"閾",
	"驟雨",
	"鶸",
	"神",
	"諸人往来",
	"飼育",
	"鶴次郎",
	"京都まで",
	"時間",
	"満ち欠け",
}
