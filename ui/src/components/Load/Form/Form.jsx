import React, {useEffect} from 'react';
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
import '../../App/App.css';

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

const Form = ({change,submit}) => {

    const [fromDate, setFromDate] = React.useState(new Date(new Date().getTime() - (24 * 60 * 60 * 1000)));
    const [toDate, setToDate] = React.useState(new Date());

    const [coin, setCoin] = React.useState('BTC');

    useEffect(()=>{
        const from_date = formatDate(fromDate)
        const to_date = formatDate(toDate)
        change({
            from: from_date,
            to: to_date,
            coin: coin
        })
    },[fromDate,toDate,coin])

    const loadHistory = () => {
        const from_date = formatDate(fromDate)
        const to_date = formatDate(toDate)
        submit({
            from: from_date,
            to: to_date,
            coin: coin
        })
    }

    const handleSetCoin = (event) => {
        console.log(event.target.value)
        setCoin(event.target.value);
    };

    return (
        <>
            <div className="row">
                <div className="col m2 s12 right">
                    <Button onClick={loadHistory}
                            variant="outlined">Load History</Button>
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
                            <MenuItem value="DOT">DOT</MenuItem>
                            <MenuItem value="LINK">LINK</MenuItem>
                            <MenuItem value="SOL">SOL</MenuItem>
                            <MenuItem value="KSM">KSM</MenuItem>
                            <MenuItem value="KAVA">KAVA</MenuItem>
                            <MenuItem value="AAVE">AAVE</MenuItem>
                            <MenuItem value="MATIC">MATIC</MenuItem>
                            <MenuItem value="DAI">DAI</MenuItem>
                            <MenuItem value="TRX">TRX</MenuItem>
                            <MenuItem value="XLM">XLM</MenuItem>
                            <MenuItem value="FIL">FIL</MenuItem>
                            <MenuItem value="XMR">XMR</MenuItem>
                            <MenuItem value="XTZ">XTZ</MenuItem>
                            <MenuItem value="FLOW">FLOW</MenuItem>
                            <MenuItem value="DASH">DASH</MenuItem>
                            <MenuItem value="SC">SC</MenuItem>
                            <MenuItem value="KEEP">KEEP</MenuItem>
                            <MenuItem value="REP">REP</MenuItem>
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
