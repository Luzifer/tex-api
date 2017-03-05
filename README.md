# Luzifer / tex-api

`tex-api` is a docker container being able to generate a PDF document from a LaTeX file and additional assets using a simple `curl` call.

## Usage

- Start the container at a trusted location:  
`docker run -d -p 3000:3000 -v /data/tex-api:/storage quay.io/luzifer/tex-api`
- Compile a ZIP file, send it to the API and get the result:

```bash
# zip -r letter.zip fonts main.tex sig.jpeg
  adding: fonts/ (stored 0%)
  adding: fonts/Roboto-Black.ttf (deflated 46%)
  adding: fonts/Roboto-BlackItalic.ttf (deflated 46%)
  adding: fonts/Roboto-Bold.ttf (deflated 47%)
  adding: fonts/Roboto-BoldItalic.ttf (deflated 45%)
  adding: fonts/Roboto-Italic.ttf (deflated 47%)
  adding: fonts/Roboto-Light.ttf (deflated 47%)
  adding: fonts/Roboto-LightItalic.ttf (deflated 47%)
  adding: fonts/Roboto-Medium.ttf (deflated 46%)
  adding: fonts/Roboto-MediumItalic.ttf (deflated 46%)
  adding: fonts/Roboto-Regular.ttf (deflated 46%)
  adding: fonts/Roboto-Thin.ttf (deflated 44%)
  adding: fonts/Roboto-ThinItalic.ttf (deflated 42%)
  adding: fonts/RobotoMono-Bold.ttf (deflated 41%)
  adding: fonts/RobotoMono-BoldItalic.ttf (deflated 40%)
  adding: fonts/RobotoMono-Italic.ttf (deflated 39%)
  adding: fonts/RobotoMono-Light.ttf (deflated 42%)
  adding: fonts/RobotoMono-LightItalic.ttf (deflated 41%)
  adding: fonts/RobotoMono-Medium.ttf (deflated 41%)
  adding: fonts/RobotoMono-MediumItalic.ttf (deflated 40%)
  adding: fonts/RobotoMono-Regular.ttf (deflated 41%)
  adding: fonts/RobotoMono-Thin.ttf (deflated 41%)
  adding: fonts/RobotoMono-ThinItalic.ttf (deflated 40%)
  adding: main.tex (deflated 58%)
  adding: sig.jpeg (deflated 3%)

# curl -sSL -H 'Accept: application/tar' --data-binary @letter.zip localhost:3000/job | tar -xvf -
main.log
main.pdf
```

What happened here is we packed all assets required for generating the letter into the ZIP archive, pushed it to the API, waited for it to build a TAR and extracted the resulting files from it.

## API

```
POST  /job                  Create a new processing job (request body is expected
                            to be a ZIP file having at least one .tex file at the
                            root of the archive) and redirect to the /wait endpoint
GET   /job/{uuid}           Retrieve the status of the processing job
GET   /job/{uuid}/wait      Wait and redirect until the processing job is finished
                            or errored
GET   /job/{uuid}/download  Download the resulting archive (You may specify an
                            Accept header to select whether to receive a ZIP or
                            TAR archive.)
```
