import React from 'react';
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
import Rating from '@mui/material/Rating';
import Typography from '@mui/material/Typography';
import '../App/App.css';

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

    const [duration, setDuration] = React.useState(15)

    const [prev, setPrev] = React.useState(3)
    const [next, setNext] = React.useState(1)

    function handleSetDuration(event) {
        setDuration(event.target.value);
    }

    const handleSetCoin = (event) => {
        setCoin(event.target.value);
    };

    const runScenario = () => {

        const from_date = formatDate(fromDate)
        const to_date = formatDate(toDate)

        const headers = {
            'Content-Type': 'application/json'
        }
        fetch('http://localhost:6090/test/run?coin=' + coin + '&from=' + from_date + '&to=' + to_date + '&interval=' + duration + '&prev='+prev+'&next='+next+'',
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
                <div className="col m2 s12 right">
                    <Typography component="legend">Look ahead</Typography>
                    <Rating
                        name="simple-next"
                        value={next}
                        onChange={(event, newValue) => {
                            setNext(newValue);
                        }}
                        size="large"
                        max={10}
                    />
                </div>
                <div className="col m2 s12 right">
                    <FormControl sx={{m: 1, minWidth: 80}}>
                        <InputLabel id="demo-simple-select-label">Interval</InputLabel>
                        <Select
                            labelId="demo-simple-select-label"
                            id="demo-simple-select"
                            value={duration}
                            label="Interval"
                            onChange={handleSetDuration}
                        >
                            <MenuItem value={1}>1</MenuItem>
                            <MenuItem value={2}>2</MenuItem>
                            <MenuItem value={3}>3</MenuItem>
                            <MenuItem value={5}>5</MenuItem>
                            <MenuItem value={10}>10</MenuItem>
                            <MenuItem value={15}>15</MenuItem>
                            <MenuItem value={20}>20</MenuItem>
                            <MenuItem value={25}>25</MenuItem>
                            <MenuItem value={30}>30</MenuItem>
                        </Select>
                    </FormControl>
                </div>
                <div className="col m2 s12 left">
                    <Typography component="legend">Look back</Typography>
                    <Rating
                        name="simple-prev"
                        value={prev}
                        onChange={(event, newValue) => {
                            setPrev(newValue);
                        }}
                        size="large"
                        max={10}
                    />
                </div>
            </div>
            <div className="row">
                <div className="col m2 s12 right">
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
