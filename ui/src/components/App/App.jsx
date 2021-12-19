import React, {useState} from 'react';
import 'materialize-css/dist/css/materialize.min.css';
import './App.css';
import Form from '../Stats/Form/Form';
import Info from '../Stats/Info/Info';
import Bar from '../Stats/Bar/Bar';
import Box from '@mui/material/Box';
import Tab from '@mui/material/Tab';
import TabContext from '@mui/lab/TabContext';
import TabList from '@mui/lab/TabList';
import TabPanel from '@mui/lab/TabPanel';
import {getData} from '../../helpers/localStorage';
import Stats from "../Stats/stats";
import Train from "../Train/train";
import Load from "../Load/load";

const App = () => {
    const initialState = () => getData('data') || [];
    const [state, setState] = useState(initialState);
    const [data, setData] = useState({});

    const [value, setValue] = React.useState('1');

    const handleTabChange = (event, newValue) => {
        setValue(newValue);
    };
    // useEffect(() => {
    //   storeData('data', state);
    //   const date = state.map(obj => obj.date);
    //   const bmi = state.map(obj => obj.bmi);
    //   let newData = { date, bmi };
    //   setData(newData);
    // }, [state]);

    const handleChange = (data) => {
        console.log(data)

        const time = data.time.map((v,_) => {
            return new Date(v)
        })

        const price = data.price.map((p,_)=>{
            return {
                x : new Date(p.x),
                y: p.y
            }
        })

        const trades = data.trades.map((t,_)=>{
            return {
                x :  new Date(t.x),
                y: t.y
            }
        })

        const buy = data.trigger.buy.map((b,_) => {
            return {
                x :  new Date(b.x),
                y: b.y
            }
        })

        const sell = data.trigger.sell.map((s,_) => {
            return {
                x :  new Date(s.x),
                y: s.y
            }
        })
        console.log(trades)
        console.log(price)
        console.log(buy)
        console.log(sell)

        let newData = {
            details: data.details,
            coin: data.details[0].coin,
            time :time,
            trades: trades,
            price: price,
            buy: buy,
            sell: sell,
        };
        setData(newData);
    };

    // const handleDelete = id => {
    //   storeData('lastState', state);
    //   let newState = state.filter(i => {
    //     return i.id !== id;
    //   });
    //   setState(newState);
    // };

    const handleUndo = () => {
        setState(getData('lastState'));
    };

    return (
        <Box sx={{ width: '100%', typography: 'body1' }}>
            <TabContext value={value}>
                <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                    <TabList onChange={handleTabChange} aria-label="lab API tabs example">
                        <Tab label="Stats" value="1" />
                        <Tab label="Train" value="2" />
                        <Tab label="Load" value="3"/>
                    </TabList>
                </Box>
                <TabPanel value="1">
                    <Stats></Stats>
                </TabPanel>
                <TabPanel value="2">
                   <Train></Train>
                </TabPanel>
                <TabPanel value="3">
                    <Load></Load>
                </TabPanel>
            </TabContext>
        </Box>
    );
};

export default App;
