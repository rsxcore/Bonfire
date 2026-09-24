// Command genres builds the app icon from assets/bonfire.png and writes the
// Windows resources (icon, manifest, version info) that the Go linker embeds into the exe.
//
//	go run ./tools/genres -version 2.0.0
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
	"golang.org/x/image/draw"
)

const source = "assets/bonfire.png"

func main() {
	ver := flag.String("version", "2.0.0", "version written into the exe")
	flag.Parse()

	src, err := readPNG(source)
	if err != nil {
		log.Fatal(err)
	}
	if err := writePNG("assets/icon.png", resize(src, 256)); err != nil {
		log.Fatal(err)
	}

	var images []image.Image
	for _, size := range []int{16, 20, 24, 32, 40, 48, 64, 128, 256} {
		images = append(images, resize(src, size))
	}
	if err := writeICO("assets/bonfire.ico", images); err != nil {
		log.Fatal(err)
	}

	rs := winres.ResourceSet{}
	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		log.Fatal(err)
	}
	if err := rs.SetIcon(winres.Name("APPICON"), icon); err != nil {
		log.Fatal(err)
	}
	rs.SetManifest(winres.AppManifest{
		ExecutionLevel:      winres.AsInvoker,
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
	})

	num := strings.SplitN(*ver, "-", 2)[0] + ".0"
	var vi version.Info
	vi.SetFileVersion(num)
	vi.SetProductVersion(num)
	for k, v := range map[string]string{
		version.ProductName:      "Bonfire",
		version.FileDescription:  "Bonfire - save manager for FromSoftware games",
		version.InternalName:     "Bonfire",
		version.OriginalFilename: "Bonfire.exe",
		version.ProductVersion:   *ver,
		version.FileVersion:      *ver,
	} {
		if err := vi.Set(0x0409, k, v); err != nil {
			log.Fatal(err)
		}
	}
	rs.SetVersionInfo(vi)

	out, err := os.Create("rsrc_windows_amd64.syso")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := rs.WriteObject(out, winres.ArchAMD64); err != nil {
		log.Fatal(err)
	}
}

// resize scales src into a size×size square, keeping its aspect ratio and centring it.
func resize(src image.Image, size int) image.Image {
	b := src.Bounds()
	scale := float64(size) / float64(max(b.Dx(), b.Dy()))
	w, h := int(float64(b.Dx())*scale+0.5), int(float64(b.Dy())*scale+0.5)
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	at := image.Rect((size-w)/2, (size-h)/2, (size-w)/2+w, (size-h)/2+h)
	draw.CatmullRom.Scale(dst, at, src, b, draw.Over, nil)
	return dst
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeICO writes a standalone .ico with PNG-compressed entries (supported since Windows Vista).
func writeICO(path string, images []image.Image) error {
	var pngs [][]byte
	for _, img := range images {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		pngs = append(pngs, buf.Bytes())
	}

	var out bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&out, le, [3]uint16{0, 1, uint16(len(images))})
	offset := uint32(6 + 16*len(images))
	for i, img := range images {
		size := img.Bounds().Dx()
		dim := uint8(size)
		if size >= 256 {
			dim = 0 // 0 means 256 in the ICO format
		}
		binary.Write(&out, le, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(pngs[i])), offset})
		offset += uint32(len(pngs[i]))
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
