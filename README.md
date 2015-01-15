# sharaq

Image Transformer

## Example

`http://ix.peatix.com/smartphone/event/XXXX/cover-XXXX.jpg`

## Presets

|Name        | Description       |
|:----------:|:-----------------:|
| raw        | no transformation |
| smartphone | generic smartphone preset. max width = 600, max height = 1000 |
| tablet     | generic table preset. max width = 1000, max height = 2000 |
| iphone6    | max width = 750, max height = 1334 |
| iphone5    | max width = 640, max height = 1136 |

## Components

Reverse Proxy: Grab request, decompose URL into form that is easy for Dispatcher to understand

Dispatcher: given a URL, redirects to the proper location. If the resized image does not exist, respond with the original image location, but at the same time make a request to Guardian to create them images.

Transformer: basically, willnorris.com/go/imageproxy

Guardian: Creates or deletes resized images on S3.




