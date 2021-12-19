const headers = {
    'Content-Type': 'application/json'
}

const buildUrlParams = (data) => {
    let params = []
    Object.keys(data).map((k, _) => {
        params.push(k + "=" + data[k])
    })
    return params.join("&")
}

const Client = (endpoint) => {
    return {
        call: function (params, handle) {
            if (!handle) {
                handle = (result) => {
                    console.log(result)
                }
            }
            const urlParams = buildUrlParams(params)
            console.log(urlParams)
            fetch('http://localhost:6090/test/' + endpoint + '?' + urlParams,
                {
                    headers: headers,
                    // mode: 'no-cors',
                    timeout: 20000,
                })
                .then(response => {
                    return response.json()
                })
                .then(data => {
                    handle(data)
                })
                .catch((reason) => {
                    console.log(reason)
                });
        }
    }
}

export default Client;
