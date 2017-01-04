package transformer

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/lestrrat/sharaq/internal/bbpool"
	"github.com/lestrrat/sharaq/internal/log"
	"github.com/lestrrat/sharaq/internal/util"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Transformer is based on imageproxy by Will Norris. Code was shamelessly
// stolen from there.
type Transformer struct {
	client *http.Client // client used to fetch remote URLs
}

type TransformingTransport struct {
	transport http.RoundTripper
	client    *http.Client
}

type Result struct {
	Content     io.Writer
	ContentType string
	Size        int64
}

func New() *Transformer {
	client := &http.Client{}
	client.Transport = &TransformingTransport{
		transport: http.DefaultTransport,
		client:    client,
	}
	return &Transformer{
		client: client,
	}
}

// Transform takes a string that specifies the transformation,
// the url of the target, and populates the given result object
// if transformation was successful
func (t *Transformer) Transform(options string, u string, result *Result) error {
	if opts := ParseOptions(options); opts != emptyOptions {
		u += "#" + opts.String()
	}

	res, err := t.client.Get(u)
	if err != nil {
		return errors.Wrap(err, `failed to fetch remote image`)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.Errorf(`failed to fetch remote image: %d`, res.StatusCode)
	}

	if _, err := io.CopyN(result.Content, res.Body, res.ContentLength); err != nil {
		return errors.Wrap(err, `failed to read transformed content`)
	}
	result.ContentType = res.Header.Get("Content-Type")
	result.Size = res.ContentLength

	return nil
}

func (t *TransformingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Fragment == "" {
		// normal requests pass through
		log.Debugf(util.RequestCtx(req), "fetching remote URL: %v", req.URL)
		return t.transport.RoundTrip(req)
	}

	u := *req.URL
	u.Fragment = ""
	resp, err := t.client.Get(u.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	img := bbpool.Get()
	defer bbpool.Release(img)

	opt := ParseOptions(req.URL.Fragment)
	if err := transform(img, resp.Body, opt); err != nil {
		return nil, err
	}

	buf := bbpool.Get()
	defer bbpool.Release(buf)

	// replay response with transformed image and updated content length
	fmt.Fprintf(buf, "%s %s\r\n", resp.Proto, resp.Status)
	resp.Header.WriteSubset(buf, map[string]bool{"Content-Length": true})
	fmt.Fprintf(buf, "Content-Length: %d\r\n\r\n", img.Len())

	if _, err := img.WriteTo(buf); err != nil {
		return nil, errors.Wrap(err, `failed to write transformed image`)
	}

	// This buffer may NOT be allocated from the bufferpool
	// because it is then used by bufio.Reader, which doesn't
	// immediately finish reading.
	//
	// Without this, we run the risk of releasing the buffer
	// before bufio gets the chance to read it all
	//
	// Furthermore, we copy the bytes here (by creating a string)
	// to make absolutely sure that we don't accidentally reuse
	// the buffer somewhere
	outbuf := bufio.NewReader(bytes.NewBufferString(buf.String()))

	return http.ReadResponse(outbuf, req)
}

// URLError reports a malformed URL error.
type URLError struct {
	Message string
	URL     *url.URL
}

func (e URLError) Error() string {
	return fmt.Sprintf("malformed URL %q: %s", e.URL, e.Message)
}

// Options specifies transformations to be performed on the requested image.
type Options struct {
	// See ParseOptions for interpretation of Width and Height values
	Width  float64
	Height float64

	// If true, resize the image to fit in the specified dimensions.  Image
	// will not be cropped, and aspect ratio will be maintained.
	Fit bool

	// Rotate image the specified degrees counter-clockwise.  Valid values
	// are 90, 180, 270.
	Rotate int

	FlipVertical   bool
	FlipHorizontal bool
}

var emptyOptions = Options{}

func (o Options) String() string {
	buf := bbpool.Get()
	defer bbpool.Release(buf)

	fmt.Fprintf(buf, "%vx%v", o.Width, o.Height)
	if o.Fit {
		buf.WriteString(",fit")
	}
	if o.Rotate != 0 {
		fmt.Fprintf(buf, ",r%d", o.Rotate)
	}
	if o.FlipVertical {
		buf.WriteString(",fv")
	}
	if o.FlipHorizontal {
		buf.WriteString(",fh")
	}
	return buf.String()
}

// ParseOptions parses str as a list of comma separated transformation options.
// The following options can be specified in any order:
//
// Size and Cropping
//
// The size option takes the general form "{width}x{height}", where width and
// height are numbers. Integer values greater than 1 are interpreted as exact
// pixel values. Floats between 0 and 1 are interpreted as percentages of the
// original image size. If either value is omitted or set to 0, it will be
// automatically set to preserve the aspect ratio based on the other dimension.
// If a single number is provided (with no "x" separator), it will be used for
// both height and width.
//
// Depending on the size options specified, an image may be cropped to fit the
// requested size. In all cases, the original aspect ratio of the image will be
// preserved; imageproxy will never stretch the original image.
//
// When no explicit crop mode is specified, the following rules are followed:
//
// - If both width and height values are specified, the image will be scaled to
// fill the space, cropping if necessary to fit the exact dimension.
//
// - If only one of the width or height values is specified, the image will be
// resized to fit the specified dimension, scaling the other dimension as
// needed to maintain the aspect ratio.
//
// If the "fit" option is specified together with a width and height value, the
// image will be resized to fit within a containing box of the specified size.
// As always, the original aspect ratio will be preserved. Specifying the "fit"
// option with only one of either width or height does the same thing as if
// "fit" had not been specified.
//
// Rotation and Flips
//
// The "r{degrees}" option will rotate the image the specified number of
// degrees, counter-clockwise. Valid degrees values are 90, 180, and 270.
//
// The "fv" option will flip the image vertically. The "fh" option will flip
// the image horizontally. Images are flipped after being rotated.
//
// Examples
//
// 	0x0       - no resizing
// 	200x      - 200 pixels wide, proportional height
// 	0.15x     - 15% original width, proportional height
// 	x100      - 100 pixels tall, proportional width
// 	100x150   - 100 by 150 pixels, cropping as needed
// 	100       - 100 pixels square, cropping as needed
// 	150,fit   - scale to fit 150 pixels square, no cropping
// 	100,r90   - 100 pixels square, rotated 90 degrees
// 	100,fv,fh - 100 pixels square, flipped horizontal and vertical
func ParseOptions(str string) Options {
	var options Options

	for _, opt := range strings.Split(str, ",") {
		switch {
		case opt == "fit":
			options.Fit = true
		case opt == "fv":
			options.FlipVertical = true
		case opt == "fh":
			options.FlipHorizontal = true
		case len(opt) > 2 && opt[:1] == "r":
			options.Rotate, _ = strconv.Atoi(opt[1:])
		case strings.ContainsRune(opt, 'x'):
			size := strings.SplitN(opt, "x", 2)
			if w := size[0]; w != "" {
				options.Width, _ = strconv.ParseFloat(w, 64)
			}
			if h := size[1]; h != "" {
				options.Height, _ = strconv.ParseFloat(h, 64)
			}
		default:
			if size, err := strconv.ParseFloat(opt, 64); err == nil {
				options.Width = size
				options.Height = size
			}
		}
	}

	return options
}

// Request is an imageproxy request which includes a remote URL of an image to
// proxy, and an optional set of transformations to perform.
type Request struct {
	URL     *url.URL // URL of the image to proxy
	Options Options  // Image transformation to perform
}

// NewRequest parses an http.Request into an imageproxy Request.  Options and
// the remote image URL are specified in the request path, formatted as:
// /{options}/{remote_url}.  Options may be omitted, so a request path may
// simply contian /{remote_url}.  The remote URL must be an absolute "http" or
// "https" URL, should not be URL encoded, and may contain a query string.
//
// Assuming an imageproxy server running on localhost, the following are all
// valid imageproxy requests:
//
// 	http://localhost/100x200/http://example.com/image.jpg
// 	http://localhost/100x200,r90/http://example.com/image.jpg?foo=bar
// 	http://localhost//http://example.com/image.jpg
// 	http://localhost/http://example.com/image.jpg
func NewRequest(r *http.Request) (*Request, error) {
	var err error
	req := new(Request)

	path := r.URL.Path[1:] // strip leading slash
	req.URL, err = url.Parse(path)
	if err != nil || !req.URL.IsAbs() {
		// first segment should be options
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			return nil, URLError{"too few path segments", r.URL}
		}

		req.URL, err = url.Parse(parts[1])
		if err != nil {
			return nil, URLError{fmt.Sprintf("unable to parse remote URL: %v", err), r.URL}
		}

		req.Options = ParseOptions(parts[0])
	}

	if !req.URL.IsAbs() {
		return nil, URLError{"must provide absolute remote URL", r.URL}
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, URLError{"remote URL must have http or https scheme", r.URL}
	}

	// query string is always part of the remote URL
	req.URL.RawQuery = r.URL.RawQuery
	return req, nil
}

// compression quality of resized jpegs
const jpegQuality = 95

// resample filter used when resizing images
var resampleFilter = imaging.Lanczos

// Transform the provided image.  img should contain the raw bytes of an
// encoded image in one of the supported formats (gif, jpeg, or png).  The
// bytes of a similarly encoded image is returned.
func transform(dst io.Writer, img io.Reader, opt Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if opt.String() == emptyOptions.String() { // XXX WTF. This is bad. fix it
		// bail if no transformation was requested
		n, err := io.Copy(dst, img)
		if err != nil {
			return errors.Wrap(err, `failed to copy image`)
		}
		log.Debugf(ctx, "empty options, copied %d bytes", n)
		return nil
	}

	log.Debugf(ctx, "Transforming image with rule '%#v'", opt)
	// decode image
	m, format, err := image.Decode(img)
	if err != nil {
		return errors.Wrap(err, `failed to decode image`)
	}

	m = transformImage(m, opt)

	// encode image
	switch format {
	case "gif":
		gif.Encode(dst, m, nil)
	case "jpeg":
		jpeg.Encode(dst, m, &jpeg.Options{Quality: jpegQuality})
	case "png":
		png.Encode(dst, m)
	}

	return nil
}

// transformImage modifies the image m based on the transformations specified
// in opt.
func transformImage(m image.Image, opt Options) image.Image {
	// convert percentage width and height values to absolute values
	imgW := m.Bounds().Max.X - m.Bounds().Min.X
	imgH := m.Bounds().Max.Y - m.Bounds().Min.Y
	var w, h int
	if 0 < opt.Width && opt.Width < 1 {
		w = int(float64(imgW) * opt.Width)
	} else if opt.Width < 0 {
		w = 0
	} else {
		w = int(opt.Width)
	}
	if 0 < opt.Height && opt.Height < 1 {
		h = int(float64(imgH) * opt.Height)
	} else if opt.Height < 0 {
		h = 0
	} else {
		h = int(opt.Height)
	}

	// never resize larger than the original image
	if w > imgW {
		w = imgW
	}
	if h > imgH {
		h = imgH
	}

	// resize
	if w != 0 || h != 0 {
		if opt.Fit {
			m = imaging.Fit(m, w, h, resampleFilter)
		} else {
			if w == 0 || h == 0 {
				m = imaging.Resize(m, w, h, resampleFilter)
			} else {
				m = imaging.Thumbnail(m, w, h, resampleFilter)
			}
		}
	}

	// flip
	if opt.FlipVertical {
		m = imaging.FlipV(m)
	}
	if opt.FlipHorizontal {
		m = imaging.FlipH(m)
	}

	// rotate
	switch opt.Rotate {
	case 90:
		m = imaging.Rotate90(m)
	case 180:
		m = imaging.Rotate180(m)
	case 270:
		m = imaging.Rotate270(m)
	}

	return m
}
