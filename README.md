# sharaq

Image Transformer

## Example

`http://sharaq.yourcompany.com/preset/event/XXXX/cover-XXXX.jpg`

## Presets

|Name        | Description       |
|:----------:|:-----------------:|
| TBD        | TBD               |

## Components

Reverse Proxy: Grab request, decompose URL into form that is easy for Dispatcher to understand

Dispatcher: given a URL, redirects to the proper location. If the resized image does not exist, respond with the original image location, but at the same time make a request to Guardian to create them images.

Transformer: basically, willnorris.com/go/imageproxy

Guardian: Creates or deletes resized images on S3.




