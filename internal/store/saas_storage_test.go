package store

import (
	"path/filepath"
	"testing"
)

func TestSafeStorageObjectPath(t *testing.T) {
	root := t.TempDir()
	target, ok := safeStorageObjectPath(root, "2026/0704/file.png")
	if !ok {
		t.Fatal("expected relative upload path to be accepted")
	}
	if target != filepath.Join(root, "2026", "0704", "file.png") {
		t.Fatalf("target = %q", target)
	}

	tests := []string{
		"",
		".",
		"..",
		"../escape.png",
		"nested/../../escape.png",
		filepath.Join(root, "absolute.png"),
	}
	for _, value := range tests {
		if target, ok := safeStorageObjectPath(root, value); ok {
			t.Fatalf("path %q accepted as %q", value, target)
		}
	}
}

func TestMediumStoragePathsFromContent(t *testing.T) {
	paths := mediumStoragePathsFromContent(`{
		"imagePath": "images/a.png",
		"voicePath": "https://cdn.example.com/voice.mp3",
		"videoPath": "../escape.mp4",
		"filePath": "files/a.pdf"
	}`)
	if len(paths) != 2 || paths[0] != "images/a.png" || paths[1] != "files/a.pdf" {
		t.Fatalf("paths = %#v", paths)
	}

	paths = mediumStoragePathsFromContent(`{"imagePath":"images/a.png","filePath":"images/a.png"}`)
	if len(paths) != 1 || paths[0] != "images/a.png" {
		t.Fatalf("deduped paths = %#v", paths)
	}
}

func TestBatchSendStoragePathsFromContent(t *testing.T) {
	paths := batchSendStoragePathsFromContent(`[
		{"msgType":"text","content":"hello"},
		{"msgType":"image","pic_url":"batch/contact.png"},
		{"msgType":"link","pic_url":"https://cdn.example.com/link.png"},
		{"msgType":"miniprogram","pic_url":"batch/mini.png"},
		{"msgType":"image","pic_url":"../escape.png"},
		{"msgType":"image","pic_url":"batch/contact.png"}
	]`)
	if len(paths) != 2 || paths[0] != "batch/contact.png" || paths[1] != "batch/mini.png" {
		t.Fatalf("paths = %#v", paths)
	}

	if paths := batchSendStoragePathsFromContent(`{"msgType":"image","pic_url":"not-array.png"}`); len(paths) != 0 {
		t.Fatalf("expected invalid batch content to be ignored, got %#v", paths)
	}
}

func TestChannelCodeWelcomeStoragePathsFromContent(t *testing.T) {
	paths := channelCodeWelcomeStoragePathsFromContent(`{
		"scanCodePush": 2,
		"messageDetail": [
			{"type": 1, "mediumId": 900001, "content": {"imagePath": "channelCode/welcome-image.png"}},
			{"type": 2, "detail": [
				{"timeSlot": [
					{"pic_url": "http://api.example.com/static/channelCode/time-slot.png"},
					{"picUrl": "https://cdn.example.com/remote.png"},
					{"image": "../escape.png"},
					{"link": "pages/index/not-a-storage-path"}
				]}
			]},
			{"type": 3, "linkPic": "static/channelCode/link.png"}
		]
	}`)
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"channelCode/welcome-image.png": {},
		"channelCode/time-slot.png":     {},
		"channelCode/link.png":          {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}

	if paths := channelCodeWelcomeStoragePathsFromContent(`{"messageDetail":{"pic_url":"not-array.png"}}`); len(paths) != 1 || paths[0] != "not-array.png" {
		t.Fatalf("expected object payload to be scanned, got %#v", paths)
	}
}

func TestRoomWelcomeStoragePathsFromContent(t *testing.T) {
	paths := roomWelcomeStoragePathsFromContent(`{
		"pic": "image/roomWelcome/a.jpg",
		"pic_url": "https://wx.example.com/a.jpg"
	}`)
	if len(paths) != 1 || paths[0] != "image/roomWelcome/a.jpg" {
		t.Fatalf("paths = %#v", paths)
	}

	if paths := roomWelcomeStoragePathsFromContent(`{"pic":"../escape.jpg"}`); len(paths) != 0 {
		t.Fatalf("expected unsafe path to be ignored, got %#v", paths)
	}
}

func TestRoomTagPullStoragePathsFromRooms(t *testing.T) {
	paths := roomTagPullStoragePathsFromRooms(`[
		{"id": 1, "image": "roomTagPull/a.png", "wx_image": "https://wecom.example/a.png"},
		{"id": 2, "image": "https://cdn.example.com/b.png"},
		{"id": 3, "image": "../escape.png"},
		{"id": 4, "image": "roomTagPull/a.png"},
		{"id": 5, "image": "roomTagPull/c.png"}
	]`)
	if len(paths) != 2 || paths[0] != "roomTagPull/a.png" || paths[1] != "roomTagPull/c.png" {
		t.Fatalf("paths = %#v", paths)
	}

	if paths := roomTagPullStoragePathsFromRooms(`{"image":"not-array.png"}`); len(paths) != 0 {
		t.Fatalf("expected invalid rooms payload to be ignored, got %#v", paths)
	}
}

func TestWorkRoomAutoPullStoragePathsFromRooms(t *testing.T) {
	paths := workRoomAutoPullStoragePathsFromRooms(`[
		{"roomId": 1, "roomQrcodeUrl": "autoPull/a.png", "longRoomQrcodeUrl": "https://api.example.com/autoPull/a.png"},
		{"roomId": 2, "room_qrcode_url": "autoPull/b.png", "long_room_qrcode_url": "../escape.png"},
		{"roomId": 3, "roomQrcodeUrl": "autoPull/a.png"}
	]`)
	if len(paths) != 2 || paths[0] != "autoPull/a.png" || paths[1] != "autoPull/b.png" {
		t.Fatalf("paths = %#v", paths)
	}

	if paths := workRoomAutoPullStoragePathsFromRooms(`{"roomQrcodeUrl":"not-array.png"}`); len(paths) != 0 {
		t.Fatalf("expected invalid rooms payload to be ignored, got %#v", paths)
	}
}

func TestShopCodeStoragePathsFromFields(t *testing.T) {
	paths := shopCodeStoragePathsFromFields(
		`[{"url":"shopCode/employee.png"},{"url":"https://cdn.example.com/remote.png"},{"description":"not-a-path.png"}]`,
		`[{"roomQrcodeUrl":"static/shopCode/room.png"},{"qrcode":"../escape.png"},{"routePath":"pages/shop/index"}]`,
		`"shopCode/root.png"`,
	)
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"shopCode/employee.png": {},
		"shopCode/room.png":     {},
		"shopCode/root.png":     {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestShopCodePageSettingStoragePathsFromFields(t *testing.T) {
	paths := shopCodePageSettingStoragePathsFromFields(
		`{"logo":"shopCode/page-logo.png","poster":"https://cdn.example.com/remote.png","guide":"not-a-path.png","image":"../escape.png"}`,
		"http://api.example.com/static/shopCode/page-poster.png",
	)
	if len(paths) != 2 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"shopCode/page-logo.png":   {},
		"shopCode/page-poster.png": {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestStoragePathsRemoved(t *testing.T) {
	removed := storagePathsRemoved(
		[]string{"images/old.png", "images/keep.png", "images/old.png", "https://cdn.example.com/remote.png"},
		[]string{"images/keep.png", "images/new.png"},
	)
	if len(removed) != 1 || removed[0] != "images/old.png" {
		t.Fatalf("removed = %#v", removed)
	}

	removed = storagePathsRemoved(
		[]string{"images/keep.png", "files/keep.pdf"},
		[]string{"files/keep.pdf", "images/keep.png"},
	)
	if len(removed) != 0 {
		t.Fatalf("expected no removed paths, got %#v", removed)
	}
}

func TestContactBatchAddStoragePathsFromFileURLs(t *testing.T) {
	paths := contactBatchAddStoragePathsFromFileURLs([]string{
		"contactBatchAdd/import/2026/0706/a.csv",
		"http://api.example.com/static/contactBatchAdd/import/2026/0706/b.xlsx",
		"https://cdn.example.com/remote.csv",
		"../escape.csv",
		"static/contactBatchAdd/import/2026/0706/c.csv",
		"contactBatchAdd/import/2026/0706/a.csv",
	})
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"contactBatchAdd/import/2026/0706/a.csv":  {},
		"contactBatchAdd/import/2026/0706/b.xlsx": {},
		"contactBatchAdd/import/2026/0706/c.csv":  {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestRoomFissionStoragePathsFromFields(t *testing.T) {
	paths := roomFissionStoragePathsFromFields(
		"roomFission/poster.png",
		"http://api.example.com/static/roomFission/room.png",
		"https://cdn.example.com/remote.png",
		"../escape.png",
		"static/roomFission/welcome.png",
		"roomFission/poster.png",
		"roomFission/invite.png",
	)
	if len(paths) != 4 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"roomFission/poster.png":  {},
		"roomFission/room.png":    {},
		"roomFission/welcome.png": {},
		"roomFission/invite.png":  {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestRadarStoragePathsFromFields(t *testing.T) {
	paths := radarStoragePathsFromFields(
		"radar/cover.png",
		"http://api.example.com/static/radar/doc.pdf",
		"https://cdn.example.com/remote.png",
		"../escape.pdf",
		"static/radar/inline.png",
		"radar/cover.png",
	)
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"radar/cover.png":  {},
		"radar/doc.pdf":    {},
		"radar/inline.png": {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestRoomClockInStoragePathsFromFields(t *testing.T) {
	paths := roomClockInStoragePathsFromFields(
		"clockIn/employee.png",
		"http://api.example.com/static/clockIn/backup.png",
		"https://cdn.example.com/remote.png",
		"../escape.png",
		"static/clockIn/inline.png",
		"clockIn/employee.png",
	)
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"clockIn/employee.png": {},
		"clockIn/backup.png":   {},
		"clockIn/inline.png":   {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestRoomInfinitePullStoragePathsFromFields(t *testing.T) {
	paths := roomInfinitePullStoragePathsFromFields(
		"roomInfinite/avatar.png",
		"http://api.example.com/static/roomInfinite/logo.png",
		`[
			{"qrcode":"roomInfinite/qrcode.png","upper_limit":200,"status":1},
			{"qrCode":"static/roomInfinite/qrcode2.png"},
			{"qr_code":"https://cdn.example.com/remote.png"},
			{"pic_url":"../escape.png"},
			{"roomQrcodeUrl":"roomInfinite/qrcode.png"}
		]`,
	)
	if len(paths) != 4 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"roomInfinite/avatar.png":  {},
		"roomInfinite/logo.png":    {},
		"roomInfinite/qrcode.png":  {},
		"roomInfinite/qrcode2.png": {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestLotteryStoragePathsFromJSON(t *testing.T) {
	paths := appendLotteryStoragePathsFromJSON(nil, `[
		{"name":"一等奖","image":"lottery/prize.png","cover_url":"http://api.example.com/static/lottery/cover.png"},
		{"name":"二等奖","image":"https://cdn.example.com/remote.png","avatar":"../escape.png"},
		{"name":"三等奖","qrcode":"static/lottery/qrcode.png","description":"not-a-path.png"},
		{"name":"重复奖品","image":"lottery/prize.png"}
	]`)
	paths = appendLotteryStoragePathsFromJSON(paths, `{"corpName":"极义","logo":"lottery/logo.png","url":"https://example.com/activity"}`)
	paths = uniqueStorageRelativePaths(paths)
	if len(paths) != 4 {
		t.Fatalf("paths = %#v", paths)
	}
	expected := map[string]struct{}{
		"lottery/prize.png":  {},
		"lottery/cover.png":  {},
		"lottery/qrcode.png": {},
		"lottery/logo.png":   {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q in %#v", path, paths)
		}
	}
}

func TestWorkFissionStoragePathsFromJSON(t *testing.T) {
	paths := appendWorkFissionStoragePathsFromJSON(nil, `{
		"image": "fission/push.png",
		"pic_url": "https://cdn.example.com/remote.png",
		"link": {
			"image": "fission/link.png",
			"title": "not-a-path"
		},
		"applets": [{
			"image": "../escape.png"
		}]
	}`)
	paths = uniqueStorageRelativePaths(paths)
	pathSet := map[string]struct{}{}
	for _, path := range paths {
		pathSet[path] = struct{}{}
	}
	if len(paths) != 2 {
		t.Fatalf("paths = %#v", paths)
	}
	if _, ok := pathSet["fission/push.png"]; !ok {
		t.Fatalf("paths = %#v", paths)
	}
	if _, ok := pathSet["fission/link.png"]; !ok {
		t.Fatalf("paths = %#v", paths)
	}
}
