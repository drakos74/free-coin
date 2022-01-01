import React, {useEffect, useState} from 'react';
import TextField from '@mui/material/TextField';
import AdapterDateFns from '@mui/lab/AdapterDateFns';
import LocalizationProvider from '@mui/lab/LocalizationProvider';
import DatePicker from '@mui/lab/DatePicker';
import PropTypes from 'prop-types';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import FormControl from '@mui/material/FormControl';
import Select from '@mui/material/Select';
import Button from '@mui/material/Button';
import Slider from '@mui/material/Slider';
import Typography from '@mui/material/Typography';
import '../../App/App.css';
import Client from "../../../api/client";
import {Autocomplete} from "@mui/lab";

const initialValues = {
    weight: '',
    height: '',
    date: ''
}

const formatDate = function (date) {
    const yyyy = date.getUTCFullYear()
    let mm = date.getUTCMonth() + 1
    mm = (mm > 9 ? '' : '0') + mm
    let dd = date.getUTCDate()
    dd = (dd > 9 ? '' : '0') + dd
    return yyyy + '_' + mm + '_' + dd + 'T00'
}

const Form = ({change}) => {

    const [fromDate, setFromDate] = React.useState(new Date(new Date().getTime() - (24 * 60 * 60 * 1000)));
    const [toDate, setToDate] = React.useState(new Date());

    const [coin, setCoin] = React.useState('BTC');

    const [precision, setPrecision] = React.useState(0.51);

    const [size, setSize] = React.useState(100);
    const [bufferSize, setBufferSize] = React.useState(50);
    const [features, setFeatures] = React.useState(3);

    const [lookBack, setLookBack] = React.useState(9);
    const [lookAhead, setLookAhead] = React.useState(1);
    const [gap, setGap] = React.useState(0.75);

    const [model, setModel] = React.useState([]);
    const [models, setModels] = useState([]);

    useEffect(() => {
        Client("models").call({}, (data) => {
            let models = data.map((model, _) => {
                let parts = model.split("_")
                return {
                    coin: parts[0],
                    accuracy: parseFloat(parts[3]),
                    title: model,
                    duration: parts[1],
                }
            })
            setModels(models)
        })
    }, [""])

    const handleSetCoin = (event) => {
        setCoin(event.target.value);
    };

    const runScenario = () => {

        const from_date = formatDate(fromDate)
        const to_date = formatDate(toDate)

        const headers = {
            'Content-Type': 'application/json'
        }
        fetch('http://localhost:6090/test/train?coin=' + coin +
            '&from=' + from_date +
            '&to=' + to_date +
            '&' + model.map((m) => {
                return "model=" + m
            }).join("&") +
            '&precision=' + precision +
            '&size=' + size +
            '&buffer=' + bufferSize +
            '&look_back=' + lookBack +
            '&look_ahead=' + lookAhead +
            '&gap=' + gap +
            '&features=' + features,
            {
                headers: headers,
                // mode: 'no-cors',
                timeout: 20000,
            })
            .then(response => {
                return response.json()
            })
            .then(data => {
                change(data)
            })
            .catch((reason) => {
                console.log(reason)
            });
    };

    const trainScenario = () => {

        const from_date = formatDate(fromDate)
        const to_date = formatDate(toDate)

        const headers = {
            'Content-Type': 'application/json'
        }
        fetch('http://localhost:6090/test/train?coin=' + coin +
            '&from=' + from_date +
            '&to=' + to_date,
            {
                headers: headers,
                // mode: 'no-cors',
                timeout: 20000,
            })
            .then(response => {
                return response.json()
            })
            .then(data => {
                change(data)
            })
            .catch((reason) => {
                console.log(reason)
            });
    };

    return (
        <>
            <div className="row">
                <div className="col m3 s12 left">
                    <Autocomplete
                        id="grouped-demo"
                        multiple
                        options={models.sort((a, b) => b.accuracy - a.accuracy)}
                        groupBy={(option) => option.coin}
                        getOptionLabel={(option) => option.title}
                        onChange={(_, v) => {
                            setModel(v.map((vv => vv.title)))
                        }} sx={{width: 300}}
                        renderInput={(params) => <TextField {...params} label="Models"/>}
                    />
                </div>
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Precision Threshold
                    </Typography>
                    <Slider
                        value={precision}
                        aria-label="Precision Threshold"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setPrecision(event.target.value)
                        }}
                        step={0.1}
                        min={0}
                        max={1}
                    />
                </div>
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Buffer Size
                    </Typography>
                    <Slider
                        value={bufferSize}
                        aria-label="Buffer Size"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setBufferSize(event.target.value)
                        }}
                        step={1}
                        min={10}
                        max={100}
                    />
                </div>
            </div>
            <div className="row">
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Model Size
                    </Typography>
                    <Slider
                        value={size}
                        aria-label="Model Size"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setSize(event.target.value)
                        }}
                        step={1}
                        min={10}
                        max={1000}
                    />
                </div>
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Features
                    </Typography>
                    <Slider
                        value={features}
                        aria-label="Features"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setFeatures(event.target.value)
                        }}
                        step={1}
                        min={1}
                        max={10}
                    />
                </div>
            </div>
            <div className="row">
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Look Back
                    </Typography>
                    <Slider
                        value={lookBack}
                        aria-label="Look Back"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setLookBack(event.target.value)
                        }}
                        step={1}
                        min={3}
                        max={30}
                    />
                </div>
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Look Ahead
                    </Typography>
                    <Slider
                        value={lookAhead}
                        aria-label="Look Ahead"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setLookAhead(event.target.value)
                        }}
                        step={1}
                        min={1}
                        max={30}
                    />
                </div>
                <div className="col m3 s12 right">
                    <Typography variant="h8" component="span" sx={{flexGrow: 1}}>
                        Gap
                    </Typography>
                    <Slider
                        value={gap}
                        aria-label="Gap"
                        valueLabelDisplay="auto"
                        onChange={(event) => {
                            setGap(event.target.value)
                        }}
                        step={0.05}
                        min={0.5}
                        max={1}
                    />
                </div>
            </div>
            <div className="row">
                <div className="col m2 s12 right">
                    <Button onClick={trainScenario}
                            variant="outlined">Train Model</Button>
                    <Button onClick={runScenario}
                            variant="outlined">Run Scenario</Button>
                </div>
                <div className="col m3 s12 right">
                    <LocalizationProvider dateAdapter={AdapterDateFns}>
                        <DatePicker
                            label="To"
                            value={toDate}
                            onChange={(newValue) => {
                                setToDate(newValue);
                            }}
                            renderInput={(params) => <TextField {...params} />}
                        />
                    </LocalizationProvider>
                </div>
                <div className="col m3 s12 right">
                    <LocalizationProvider dateAdapter={AdapterDateFns}>
                        <DatePicker
                            label="From"
                            value={fromDate}
                            onChange={(newValue) => {
                                setFromDate(newValue);
                            }}
                            renderInput={(params) => <TextField {...params} />}
                        />
                    </LocalizationProvider>
                </div>
                <div className="col m2 s12 right">
                    <FormControl sx={{m: 1, minWidth: 80}}>
                        <InputLabel id="coin-select-label">Coin</InputLabel>
                        <Select
                            labelId="coin-select-label"
                            id="coin-select"
                            value={coin}
                            label="Coin"
                            autoWidth
                            onChange={handleSetCoin}
                        >
                            <MenuItem value="BTC">BTC</MenuItem>
                            <MenuItem value="ETH">ETH</MenuItem>
                        </Select>
                    </FormControl>
                </div>
            </div>
        </>
    );
};

Form.propTypes = {
    change: PropTypes.func.isRequired
};

export default Form;
